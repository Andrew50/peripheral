"""
DataFrame Strategy Examples
Examples of strategies that take DataFrames and return instances
"""

# Example 1: Gap Up Strategy
GAP_UP_STRATEGY = '''
def gap_up_strategy(df):
    """Find stocks that gap up more than 3% with volume confirmation"""
    instances = []
    
    # Filter for stocks with gap data
    df_filtered = df[df['gap_pct'].notna() & (df['gap_pct'] > 3.0)]
    
    # Add volume confirmation
    df_filtered = df_filtered[df_filtered['volume_ratio'] > 1.5]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'gap_percent': round(row['gap_pct'], 2),
            'volume_ratio': round(row['volume_ratio'], 2),
            'score': min(1.0, (row['gap_pct'] / 10.0) + (row['volume_ratio'] / 5.0)),
            'message': f"{row['ticker']} gapped up {row['gap_pct']:.1f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
'''

# Example 2: RSI Oversold Strategy  
RSI_OVERSOLD_STRATEGY = '''
def rsi_oversold_strategy(df):
    """Find oversold stocks with RSI < 30"""
    instances = []
    
    # Filter for oversold conditions
    df_filtered = df[(df['rsi'] < 30) & (df['rsi'].notna())]
    
    # Additional filter: must be above 50-day SMA for trend
    df_filtered = df_filtered[(df_filtered['close'] > df_filtered['sma_50']) & (df_filtered['sma_50'].notna())]
    
    for _, row in df_filtered.iterrows():
        instances.append({
            'ticker': row['ticker'],
            'date': str(row['date']),
            'signal': True,
            'rsi': round(row['rsi'], 2),
            'price': row['close'],
            'sma_50': round(row['sma_50'], 2),
            'score': (30 - row['rsi']) / 30,  # Lower RSI = higher score
            'message': f"{row['ticker']} oversold with RSI {row['rsi']:.1f}"
        })
    
    return instances
'''

# Example 3: Simple MACD Crossover
MACD_CROSSOVER_STRATEGY = '''
def macd_crossover_strategy(df):
    """MACD bullish crossover strategy"""
    instances = []
    
    # Sort by ticker and date to ensure proper order
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate previous MACD values
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
            'date': str(row['date']),
            'signal': True,
            'macd': round(row['macd'], 4),
            'macd_signal': round(row['macd_signal'], 4),
            'macd_histogram': round(row['macd_histogram'], 4),
            'price': row['close'],
            'score': min(1.0, abs(row['macd_histogram']) * 10),
            'message': f"{row['ticker']} MACD bullish crossover at ${row['close']:.2f}"
        })
    
    return instances
'''

# All strategies dictionary for easy access
DATAFRAME_STRATEGIES = {
    'gap_up': GAP_UP_STRATEGY,
    'rsi_oversold': RSI_OVERSOLD_STRATEGY,
    'macd_crossover': MACD_CROSSOVER_STRATEGY
}
