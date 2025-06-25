"""
Filtered Data Accessor Strategy Examples
Examples showing how to use the new filtering capabilities to efficiently fetch data
"""

# Example 1: Technology Sector Gap-Up Strategy
TECH_GAP_UP_STRATEGY = '''
def strategy():
    """Find technology stocks that gap up more than 2% with volume confirmation"""
    instances = []
    
    # Get recent bar data for technology stocks only - much more efficient!
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "open", "close", "volume"], 
        min_bars=25,  # Need 20 for volume average + recent data
        filters={
            'sector': 'Technology',
            'market': 'stocks',
            'locale': 'us'
        }
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
        (df_sorted['gap_pct'] > 2.0) &
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
            'message': f"{row['ticker']} (Tech) gapped up {row['gap_pct']:.1f}% with {row['volume_ratio']:.1f}x volume"
        })
    
    return instances
'''

# Example 2: Large Cap Healthcare Momentum Strategy
LARGE_CAP_HEALTHCARE_STRATEGY = '''
def strategy():
    """Find large-cap healthcare stocks with strong momentum"""
    instances = []
    
    # Get bar data for large-cap healthcare stocks only
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"], 
        min_bars=50,
        filters={
            'sector': 'Healthcare',
            'market_cap_min': 10000000000,  # $10B+ market cap
            'locale': 'us'
        }
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Get general data for additional context
    general_data = get_general_data(
        columns=["ticker", "name", "industry", "market_cap"],
        filters={
            'sector': 'Healthcare',
            'market_cap_min': 10000000000
        }
    )
    
    import pandas as pd
    import numpy as np
    
    # Process bar data
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "open", "high", "low", "close", "volume"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate momentum indicators
    df_sorted['returns_1d'] = df_sorted.groupby('ticker')['close'].pct_change()
    df_sorted['returns_5d'] = df_sorted.groupby('ticker')['close'].pct_change(periods=5)
    df_sorted['returns_20d'] = df_sorted.groupby('ticker')['close'].pct_change(periods=20)
    
    # Calculate relative strength vs 20-day average
    df_sorted['price_vs_20d_avg'] = df_sorted['close'] / df_sorted.groupby('ticker')['close'].rolling(20).mean().reset_index(0, drop=True)
    
    # Get latest data for each ticker
    latest_data = df_sorted.groupby('ticker').tail(1)
    
    # Filter for momentum conditions
    momentum_stocks = latest_data[
        (latest_data['returns_5d'] > 0.05) &  # 5% gain in 5 days
        (latest_data['returns_20d'] > 0.10) &  # 10% gain in 20 days
        (latest_data['price_vs_20d_avg'] > 1.05)  # Above 20-day average
    ]
    
    for _, row in momentum_stocks.iterrows():
        # Get company info
        company_info = general_data[general_data['ticker'] == row['ticker']]
        company_name = company_info['name'].iloc[0] if not company_info.empty else row['ticker']
        industry = company_info['industry'].iloc[0] if not company_info.empty else 'Healthcare'
        
        momentum_score = (row['returns_5d'] + row['returns_20d']) / 2
        
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            'company_name': company_name,
            'industry': industry,
            'returns_5d': round(row['returns_5d'] * 100, 2),
            'returns_20d': round(row['returns_20d'] * 100, 2),
            'score': min(1.0, momentum_score * 2),
            'message': f"{row['ticker']} ({industry}) showing strong momentum: +{row['returns_5d']*100:.1f}% (5d), +{row['returns_20d']*100:.1f}% (20d)"
        })
    
    return instances
'''

# Example 3: NASDAQ Small Cap Value Strategy  
NASDAQ_SMALL_CAP_STRATEGY = '''
def strategy():
    """Find undervalued small-cap stocks on NASDAQ"""
    instances = []
    
    # Get bar data for NASDAQ small-cap stocks
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "close", "volume"], 
        min_bars=100,
        filters={
            'primary_exchange': 'NASDAQ',
            'market_cap_min': 300000000,   # $300M minimum
            'market_cap_max': 2000000000,  # $2B maximum (small cap range)
            'locale': 'us'
        }
    )
    
    if len(bar_data) == 0:
        return instances
    
    import pandas as pd
    
    df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
    df_sorted = df.sort_values(['ticker', 'date']).copy()
    
    # Calculate price metrics
    df_sorted['sma_50'] = df_sorted.groupby('ticker')['close'].rolling(50).mean().reset_index(0, drop=True)
    df_sorted['sma_200'] = df_sorted.groupby('ticker')['close'].rolling(200).mean().reset_index(0, drop=True)
    df_sorted['volume_avg'] = df_sorted.groupby('ticker')['volume'].rolling(50).mean().reset_index(0, drop=True)
    
    # Get latest data
    latest_data = df_sorted.groupby('ticker').tail(1)
    
    # Filter for value conditions
    value_stocks = latest_data[
        latest_data['close'].notna() &
        latest_data['sma_50'].notna() &
        latest_data['sma_200'].notna() &
        (latest_data['close'] < latest_data['sma_50'] * 0.95) &  # Below 50-day MA
        (latest_data['sma_50'] > latest_data['sma_200']) &        # But 50-day still above 200-day
        (latest_data['volume'] > latest_data['volume_avg'] * 1.2)  # Above average volume
    ]
    
    for _, row in value_stocks.iterrows():
        discount_from_sma50 = (1 - row['close'] / row['sma_50']) * 100
        
        instances.append({
            'ticker': row['ticker'],
            'timestamp': str(row['date']),
            'current_price': round(row['close'], 2),
            'discount_from_sma50': round(discount_from_sma50, 2),
            'volume_ratio': round(row['volume'] / row['volume_avg'], 2),
            'score': min(1.0, discount_from_sma50 / 10 + (row['volume'] / row['volume_avg']) / 5),
            'message': f"{row['ticker']} (NASDAQ small-cap) trading {discount_from_sma50:.1f}% below 50-day MA with elevated volume"
        })
    
    return instances
'''

# Example 4: Multi-Sector Comparison Strategy
MULTI_SECTOR_COMPARISON_STRATEGY = '''
def strategy():
    """Compare performance across different sectors"""
    instances = []
    
    sectors_to_analyze = ['Technology', 'Healthcare', 'Financials', 'Energy', 'Consumer Discretionary']
    
    for sector in sectors_to_analyze:
        # Get data for each sector separately
        bar_data = get_bar_data(
            timeframe="1d", 
            columns=["ticker", "timestamp", "close"], 
            min_bars=30,
            filters={
                'sector': sector,
                'market_cap_min': 1000000000,  # $1B+ only
                'locale': 'us'
            }
        )
        
        if len(bar_data) == 0:
            continue
            
        import pandas as pd
        import numpy as np
        
        df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close"])
        df['date'] = pd.to_datetime(df['timestamp'], unit='s').dt.date
        df_sorted = df.sort_values(['ticker', 'date']).copy()
        
        # Calculate 30-day returns for each stock
        df_sorted['returns_30d'] = df_sorted.groupby('ticker')['close'].pct_change(periods=30)
        
        # Get latest returns for each ticker
        latest_returns = df_sorted.groupby('ticker')['returns_30d'].last()
        
        # Calculate sector average
        sector_avg_return = latest_returns.mean()
        sector_median_return = latest_returns.median()
        top_performers = latest_returns.nlargest(3)
        
        # Create sector summary
        instances.append({
            'ticker': f'{sector.upper()}_SECTOR',
            'timestamp': str(pd.Timestamp.now().date()),
            'sector': sector,
            'avg_return_30d': round(sector_avg_return * 100, 2),
            'median_return_30d': round(sector_median_return * 100, 2),
            'top_performers': list(top_performers.index),
            'top_performer_returns': [round(x * 100, 2) for x in top_performers.values],
            'score': min(1.0, max(0.0, (sector_avg_return + 0.1) * 2)),  # Score based on sector performance
            'message': f"{sector} sector: avg {sector_avg_return*100:.1f}% (30d), top performer: {top_performers.index[0]} +{top_performers.iloc[0]*100:.1f}%"
        })
    
    return instances
''' 