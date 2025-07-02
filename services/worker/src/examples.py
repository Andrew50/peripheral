"""
Data Accessor Strategy Examples
Examples of strategies that use get_bar_data() and get_general_data() functions
"""

# Example 1: Gap Up Strategy with Volume Confirmation
GAP_UP_STRATEGY = '''
def strategy():
    """Find stocks that gap up more than 3% with volume confirmation"""
    instances = []
    
    # Get recent bar data with required columns only
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "open", "close", "volume"], 
        min_bars=21  # Need 20 for volume average + 1 current bar for calculation
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for easier processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "close", "volume"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate gap percentage
    df_sorted['prev_close'] = df_sorted.groupby('ticker')['close'].shift(1)
    df_sorted['gap_pct'] = ((df_sorted['open'] - df_sorted['prev_close']) / df_sorted['prev_close']) * 100
    
    # Calculate volume average and ratio
    df_sorted['volume_avg'] = df_sorted.groupby('ticker')['volume'].rolling(20).mean().reset_index(0, drop=True)
    df_sorted['volume_ratio'] = df_sorted['volume'] / df_sorted['volume_avg']
    
    # Filter for gap up conditions
    df_filtered = df_sorted[
        df_sorted['gap_pct'].notna() & 
        (df_sorted['gap_pct'] > 3.0) &
        df_sorted['volume_ratio'].notna() &
        (df_sorted['volume_ratio'] > 1.5)
    ]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            # CRITICAL: Always include important calculated indicator values
            'gap_percent': round(row['gap_pct'], 2),
            'volume_ratio': round(row['volume_ratio'], 2),
            'open_price': float(row['open']),
            'close_price': float(row['close']),
            'volume': int(row['volume']),
            'volume_avg_20': int(row['volume_avg_20']),
            'score': min(1.0, (row['gap_pct'] / 10.0) + (row['volume_ratio'] / 5.0)),
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
'''

# Example 2: RSI Oversold Strategy with Trend Filter
RSI_OVERSOLD_STRATEGY = '''
def strategy():
    """Find oversold stocks with RSI < 30 and above 50-day SMA"""
    instances = []
    
    # Get sufficient historical data for RSI and SMA calculations
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close"], 
        min_bars=50  # Need exactly 50 bars for SMA calculation (RSI needs 14, SMA needs 50, so 50 is the absolute minimum)
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for easier processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate RSI
    def calculate_rsi(prices, window=14):
        delta = prices.diff()
        gain = (delta.where(delta > 0, 0)).rolling(window=window).mean()
        loss = (-delta.where(delta < 0, 0)).rolling(window=window).mean()
        rs = gain / loss
        return 100 - (100 / (1 + rs))
    
    df_sorted['rsi'] = df_sorted.groupby('ticker')['close'].transform(lambda x: calculate_rsi(x))
    
    # Calculate 50-day SMA for trend filter
    df_sorted['sma_50'] = df_sorted.groupby('ticker')['close'].rolling(50).mean().reset_index(0, drop=True)
    
    # Filter for oversold conditions
    df_filtered = df_sorted[
        (df_sorted['rsi'] < 30) & 
        df_sorted['rsi'].notna() &
        (df_sorted['close'] > df_sorted['sma_50']) & 
        df_sorted['sma_50'].notna()
    ]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            # CRITICAL: Include ALL indicator values used in the strategy
            'rsi': round(row['rsi'], 2),
            'price': row['close'],
            'sma_50': round(row['sma_50'], 2),
            'trend_confirmation': 'bullish' if row['close'] > row['sma_50'] else 'bearish',
            'score': (30 - row['rsi']) / 30,  # Lower RSI = higher score
            'message': f"{row['ticker']} oversold with RSI {row['rsi']:.1f}"
        })
    
    return instances
'''

# Example 3: MACD Bullish Crossover Strategy
MACD_CROSSOVER_STRATEGY = '''
def strategy():
    """MACD bullish crossover strategy with custom calculation"""
    instances = []
    
    # Get historical data for MACD calculations
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close"], 
        min_bars=26  # Need exactly 26 bars for slow EMA (signal line overlaps, so 26 is absolute minimum)
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for easier processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate MACD components
    df_sorted['ema_12'] = df_sorted.groupby('ticker')['close'].ewm(span=12).mean().reset_index(0, drop=True)
    df_sorted['ema_26'] = df_sorted.groupby('ticker')['close'].ewm(span=26).mean().reset_index(0, drop=True)
    df_sorted['macd'] = df_sorted['ema_12'] - df_sorted['ema_26']
    df_sorted['macd_signal'] = df_sorted.groupby('ticker')['macd'].ewm(span=9).mean().reset_index(0, drop=True)
    df_sorted['macd_histogram'] = df_sorted['macd'] - df_sorted['macd_signal']
    
    # Calculate previous values for crossover detection
    df_sorted['macd_prev'] = df_sorted.groupby('ticker')['macd'].shift(1)
    df_sorted['macd_signal_prev'] = df_sorted.groupby('ticker')['macd_signal'].shift(1)
    
    # Find bullish crossovers (MACD crosses above signal line)
    df_filtered = df_sorted[
        (df_sorted['macd'] > df_sorted['macd_signal']) &      # Current: MACD above signal
        (df_sorted['macd_prev'] <= df_sorted['macd_signal_prev']) &  # Previous: MACD below/equal signal
        df_sorted['macd'].notna() &
        df_sorted['macd_signal'].notna() &
        df_sorted['macd_prev'].notna() &
        df_sorted['macd_signal_prev'].notna()
    ]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            # CRITICAL: Include ALL MACD components and EMAs used
            'macd': round(row['macd'], 4),
            'macd_signal': round(row['macd_signal'], 4),
            'macd_histogram': round(row['macd_histogram'], 4),
            'ema_12': round(row['ema_12'], 2),
            'ema_26': round(row['ema_26'], 2),
            'price': row['close'],
            'crossover_strength': round(row['macd'] - row['macd_signal'], 4),
            'signal_type': 'bullish_crossover',
            'score': min(1.0, abs(row['macd_histogram']) * 10),
            'message': f"{row['ticker']} MACD bullish crossover at ${row['close']:.2f}"
        })
    
    return instances
'''

# Example 4: Bollinger Band Squeeze Strategy
BOLLINGER_SQUEEZE_STRATEGY = '''
def strategy():
    """Find stocks with tight Bollinger Bands indicating potential breakout"""
    instances = []
    
    # Get historical data for Bollinger Band calculations
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close"], 
        min_bars=20  # Need exactly 20 for BB calculation (20-period moving average and std dev)
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Convert to DataFrame for easier processing
    import pandas as pd
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate Bollinger Bands
    def calculate_bollinger_bands(prices, window=20, num_std=2.0):
        rolling_mean = prices.rolling(window).mean()
        rolling_std = prices.rolling(window).std()
        upper_band = rolling_mean + (rolling_std * num_std)
        lower_band = rolling_mean - (rolling_std * num_std)
        return upper_band, rolling_mean, lower_band
    
    # Apply Bollinger Bands calculation for each ticker
    bb_results = df_sorted.groupby('ticker')['close'].apply(
        lambda x: calculate_bollinger_bands(x)
    )
    
    # Extract results
    for ticker in df_sorted['ticker'].unique():
        ticker_data = df_sorted[df_sorted['ticker'] == ticker].copy()
        if ticker in bb_results.index:
            upper, middle, lower = bb_results[ticker]
            ticker_data['bb_upper'] = upper.values
            ticker_data['bb_middle'] = middle.values
            ticker_data['bb_lower'] = lower.values
            ticker_data['bb_width'] = ticker_data['bb_upper'] - ticker_data['bb_lower']
            ticker_data['bb_width_pct'] = (ticker_data['bb_width'] / ticker_data['bb_middle']) * 100
            
            # Find squeeze conditions (narrow bands)
            squeeze_threshold = 15.0  # Band width less than 15% of middle band
            squeeze_data = ticker_data[
                ticker_data['bb_width_pct'].notna() &
                (ticker_data['bb_width_pct'] < squeeze_threshold)
            ]
            
            for _, row in squeeze_data.iterrows():
                instances.append({
                    'ticker': row['ticker'],
                    'timestamp': str(row['date']),
                    # CRITICAL: Include ALL Bollinger Band values and thresholds
                    'bb_width_pct': round(row['bb_width_pct'], 2),
                    'price': row['close'],
                    'bb_upper': round(row['bb_upper'], 2),
                    'bb_middle': round(row['bb_middle'], 2),
                    'bb_lower': round(row['bb_lower'], 2),
                    'bb_width': round(row['bb_width'], 2),
                    'position_in_bands': round((row['close'] - row['bb_lower']) / (row['bb_upper'] - row['bb_lower']), 2),
                    'score': (squeeze_threshold - row['bb_width_pct']) / squeeze_threshold,
                    'message': f"{row['ticker']} Bollinger squeeze - bands {row['bb_width_pct']:.1f}% wide"
                })
    
    return instances
'''

# Example 5: Extended Hours Gap Analysis Strategy
EXTENDED_HOURS_GAP_STRATEGY = '''
def strategy():
    """Analyze price gaps between regular hours close and extended hours activity"""
    instances = []
    
    # Get regular hours data (extended_hours=False)
    regular_data = get_bar_data(
        timeframe="1h",
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
        min_bars=10,
        extended_hours=False  # Regular trading hours only (9:30 AM - 4:00 PM ET)
    )
    
    # Get extended hours data (extended_hours=True) 
    extended_data = get_bar_data(
        timeframe="1h",
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"],
        min_bars=20,
        extended_hours=True  # Include premarket and after-hours data
    )
    
    if len(regular_data) == 0 or len(extended_data) == 0:
        return instances
    
    # Convert to DataFrames for easier processing
    import pandas as pd
    
    reg_df = pd.DataFrame(regular_data, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    ext_df = pd.DataFrame(extended_data, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    
    # Convert timestamps to datetime
    reg_df['datetime'] = pd.to_datetime(reg_df['timestamp'], unit='s')
    ext_df['datetime'] = pd.to_datetime(ext_df['timestamp'], unit='s')
    
    # Add hour column for analysis
    reg_df['hour'] = reg_df['datetime'].dt.hour
    ext_df['hour'] = ext_df['datetime'].dt.hour
    
    # Process each ticker
    for ticker in reg_df['ticker'].unique():
        ticker_reg = reg_df[reg_df['ticker'] == ticker].sort_values('datetime')
        ticker_ext = ext_df[ext_df['ticker'] == ticker].sort_values('datetime')
        
        if len(ticker_reg) < 2 or len(ticker_ext) < 2:
            continue
        
        # Get the most recent regular hours close (4:00 PM close)
        regular_close_data = ticker_reg[ticker_reg['hour'] == 15]  # 3 PM hour (closes at 4 PM)
        if len(regular_close_data) == 0:
            continue
        
        latest_regular_close = regular_close_data.iloc[-1]['close']
        
        # Get after-hours activity (after 4 PM)
        after_hours_data = ticker_ext[ticker_ext['hour'] >= 16]
        if len(after_hours_data) == 0:
            continue
        
        # Calculate after-hours price movement
        after_hours_high = after_hours_data['high'].max()
        after_hours_low = after_hours_data['low'].min()
        after_hours_volume = after_hours_data['volume'].sum()
        
        # Calculate gaps
        upside_gap_pct = ((after_hours_high - latest_regular_close) / latest_regular_close) * 100
        downside_gap_pct = ((after_hours_low - latest_regular_close) / latest_regular_close) * 100
        
        # Look for significant extended hours activity
        regular_avg_volume = ticker_reg['volume'].tail(5).mean()
        volume_ratio = after_hours_volume / regular_avg_volume if regular_avg_volume > 0 else 0
        
        # Flag significant extended hours gaps with volume
        if (abs(upside_gap_pct) > 2.0 or abs(downside_gap_pct) > 2.0) and volume_ratio > 0.3:
            instances.append({
                'ticker': ticker,
                'timestamp': str(after_hours_data.iloc[-1]['datetime'].date()),
                'regular_close': round(latest_regular_close, 2),
                'after_hours_high': round(after_hours_high, 2),
                'after_hours_low': round(after_hours_low, 2),
                'upside_gap_pct': round(upside_gap_pct, 2),
                'downside_gap_pct': round(downside_gap_pct, 2),
                'after_hours_volume': int(after_hours_volume),
                'volume_ratio': round(volume_ratio, 2),
                'score': min(1.0, (max(abs(upside_gap_pct), abs(downside_gap_pct)) / 10.0) + (volume_ratio / 2.0)),
                'message': f"{ticker} extended hours gap: {upside_gap_pct:+.1f}%/{downside_gap_pct:+.1f}% with {volume_ratio:.1f}x volume"
            })
    
    return instances
'''

# All strategies dictionary for easy access
ACCESSOR_STRATEGIES = {
    'gap_up': GAP_UP_STRATEGY,
    'rsi_oversold': RSI_OVERSOLD_STRATEGY,
    'macd_crossover': MACD_CROSSOVER_STRATEGY,
    'bollinger_squeeze': BOLLINGER_SQUEEZE_STRATEGY,
    'extended_hours_gap': EXTENDED_HOURS_GAP_STRATEGY
}
