"""
DataFrame Strategy Examples
Examples of strategies that take DataFrames with raw data and calculate their own indicators
"""

# Example 1: Gap Up Strategy with Volume Confirmation
GAP_UP_STRATEGY = '''
def gap_up_strategy(df):
    """Find stocks that gap up more than 3% with volume confirmation"""
    instances = []
    
    # Sort by ticker and date for proper calculations
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
            'gap_percent': round(row['gap_pct'], 2),
            'volume_ratio': round(row['volume_ratio'], 2),
            'score': min(1.0, (row['gap_pct'] / 10.0) + (row['volume_ratio'] / 5.0)),
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
'''

# Example 2: RSI Oversold Strategy with Trend Filter
RSI_OVERSOLD_STRATEGY = '''
def rsi_oversold_strategy(df):
    """Find oversold stocks with RSI < 30 and above 50-day SMA"""
    instances = []
    
    # Sort by ticker and date
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
            'rsi': round(row['rsi'], 2),
            'price': row['close'],
            'sma_50': round(row['sma_50'], 2),
            'score': (30 - row['rsi']) / 30,  # Lower RSI = higher score
            'message': f"{row['ticker']} oversold with RSI {row['rsi']:.1f}"
        })
    
    return instances
'''

# Example 3: MACD Bullish Crossover Strategy
MACD_CROSSOVER_STRATEGY = '''
def macd_crossover_strategy(df):
    """MACD bullish crossover strategy with custom calculation"""
    instances = []
    
    # Sort by ticker and date
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
            'macd': round(row['macd'], 4),
            'macd_signal': round(row['macd_signal'], 4),
            'macd_histogram': round(row['macd_histogram'], 4),
            'price': row['close'],
            'score': min(1.0, abs(row['macd_histogram']) * 10),
            'message': f"{row['ticker']} MACD bullish crossover at ${row['close']:.2f}"
        })
    
    return instances
'''

# Example 4: Bollinger Band Squeeze Strategy
BOLLINGER_SQUEEZE_STRATEGY = '''
def bollinger_squeeze_strategy(df):
    """Find stocks with tight Bollinger Bands indicating potential breakout"""
    instances = []
    
    # Sort by ticker and date
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
                    'bb_width_pct': round(row['bb_width_pct'], 2),
                    'price': row['close'],
                    'bb_upper': round(row['bb_upper'], 2),
                    'bb_lower': round(row['bb_lower'], 2),
                    'score': (squeeze_threshold - row['bb_width_pct']) / squeeze_threshold,
                    'message': f"{row['ticker']} Bollinger squeeze - bands {row['bb_width_pct']:.1f}% wide"
                })
    
    return instances
'''

# All strategies dictionary for easy access
DATAFRAME_STRATEGIES = {
    'gap_up': GAP_UP_STRATEGY,
    'rsi_oversold': RSI_OVERSOLD_STRATEGY,
    'macd_crossover': MACD_CROSSOVER_STRATEGY,
    'bollinger_squeeze': BOLLINGER_SQUEEZE_STRATEGY
}
