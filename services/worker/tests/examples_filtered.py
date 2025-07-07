"""
Filtered Data Accessor Strategy Examples
Examples showing how to use the new filtering capabilities to efficiently fetch data
"""

# Example 1: Technology Sector Gap Up Strategy with Volume Confirmation
TECH_GAP_UP_STRATEGY = '''
def strategy():
    """Find tech stocks that gap up more than 2% with volume confirmation"""
    instances = []
    
    # Get recent bar data for technology stocks only - much more efficient!
    bar_data = get_bar_data(
        timeframe="1d", 
        columns=["ticker", "timestamp", "open", "close", "volume"], 
        min_bars=21,  # Need 20 for volume average + 1 current bar for calculation
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
        min_bars=20,  # Need 20 for 20-day calculations (absolute minimum)
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

# Example 3: Software Companies by Size and Employee Count Strategy
SOFTWARE_COMPANIES_STRATEGY = '''
def strategy():
    """Find software companies filtered by specific criteria including employee count"""
    instances = []
    
    # Get general data for software companies with employee and share data
    general_data = get_general_data(
        columns=[
            "ticker", "name", "industry", "market_cap", 
            "total_employees", "weighted_shares_outstanding", 
            "sic_code", "sic_description"
        ],
        filters={
            'industry': 'Software',
            'market_cap_min': 1000000000,  # $1B+ market cap
            'total_employees_min': 1000,   # At least 1,000 employees
            'total_employees_max': 50000,  # No more than 50,000 employees
            'locale': 'us',
            'active': True
        }
    )
    
    if len(general_data) == 0:
        return instances
    
    # Get recent price data for these companies
    tickers = general_data['ticker'].tolist()
    bar_data = get_bar_data(
        timeframe="1d",
        filters={'tickers': tickers},
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=5
    )
    
    if len(bar_data) == 0:
        return instances
    
    import pandas as pd
    import numpy as np
    
    # Process price data
    price_df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    latest_prices = price_df.groupby('ticker').tail(1)
    
    # Merge with general data
    for _, company in general_data.iterrows():
        ticker = company['ticker']
        
        # Get latest price for this ticker
        price_data = latest_prices[latest_prices['ticker'] == ticker]
        if price_data.empty:
            continue
            
        latest_price = price_data['close'].iloc[0]
        
        # Calculate market cap per employee
        market_cap = company.get('market_cap', 0)
        employees = company.get('total_employees', 0)
        
        if employees > 0:
            market_cap_per_employee = market_cap / employees
        else:
            market_cap_per_employee = 0
        
        # Calculate enterprise value estimate (simplified)
        weighted_shares = company.get('weighted_shares_outstanding', 0)
        if weighted_shares > 0:
            shares_outstanding_est = weighted_shares
        else:
            shares_outstanding_est = market_cap / latest_price if latest_price > 0 else 0
        
        instances.append({
            'ticker': ticker,
            'timestamp': str(pd.Timestamp.now().date()),
            'company_name': company.get('name', ticker),
            'market_cap': f"${market_cap/1e9:.1f}B" if market_cap > 0 else "N/A",
            'total_employees': f"{employees:,}" if employees > 0 else "N/A",
            'market_cap_per_employee': f"${market_cap_per_employee:,.0f}" if market_cap_per_employee > 0 else "N/A",
            'weighted_shares_outstanding': f"{weighted_shares/1e6:.1f}M" if weighted_shares > 0 else "N/A",
            'sic_code': company.get('sic_code', 'N/A'),
            'sic_description': company.get('sic_description', 'N/A'),
            'latest_price': f"${latest_price:.2f}",
            'score': min(1.0, market_cap_per_employee / 1000000) if market_cap_per_employee > 0 else 0.5,
            'message': f"{ticker} - {company.get('name', ticker)}: {employees:,} employees, ${market_cap/1e9:.1f}B market cap (${market_cap_per_employee:,.0f}/employee)"
        })
    
    # Sort by market cap per employee (highest first)
    instances.sort(key=lambda x: float(x['market_cap_per_employee'].replace('$', '').replace(',', '')) if x['market_cap_per_employee'] != "N/A" else 0, reverse=True)
    
    return instances
'''

# Example 4: SIC Code Based Industry Analysis Strategy  
SIC_CODE_ANALYSIS_STRATEGY = '''
def strategy():
    """Analyze companies by specific SIC codes for targeted industry exposure"""
    instances = []
    
    # Target specific SIC codes for technology and biotech
    target_sic_codes = [
        '7372',  # Prepackaged Software
        '7373',  # Computer Integrated Systems Design
        '2834',  # Pharmaceutical Preparations
        '3674',  # Semiconductors and Related Devices
    ]
    
    all_companies = []
    
    for sic_code in target_sic_codes:
        # Get companies with specific SIC code
        companies = get_general_data(
            columns=[
                "ticker", "name", "sic_code", "sic_description", 
                "market_cap", "total_employees", "sector", "industry"
            ],
            filters={
                'sic_code': sic_code,
                'market_cap_min': 500000000,  # $500M+ market cap  
                'locale': 'us',
                'active': True
            }
        )
        
        if len(companies) > 0:
            all_companies.append(companies)
    
    if not all_companies:
        return instances
    
    import pandas as pd
    
    # Combine all companies
    combined_df = pd.concat(all_companies, ignore_index=True)
    
    # Get recent performance data
    tickers = combined_df['ticker'].tolist()
    bar_data = get_bar_data(
        timeframe="1d",
        filters={'tickers': tickers},
        columns=["ticker", "timestamp", "close", "volume"],
        min_bars=10
    )
    
    if len(bar_data) == 0:
        return instances
    
    # Process performance data
    price_df = pd.DataFrame(bar_data, columns=["ticker", "timestamp", "close", "volume"])
    
    # Calculate 10-day performance for each ticker
    price_df = price_df.sort_values(['ticker', 'timestamp'])
    price_df['returns_10d'] = price_df.groupby('ticker')['close'].pct_change(periods=9)
    latest_performance = price_df.groupby('ticker').tail(1)
    
    # Merge with company data
    for _, company in combined_df.iterrows():
        ticker = company['ticker']
        
        # Get performance data
        perf_data = latest_performance[latest_performance['ticker'] == ticker]
        returns_10d = perf_data['returns_10d'].iloc[0] if not perf_data.empty else 0
        
        # Calculate employee efficiency metric
        market_cap = company.get('market_cap', 0)
        employees = company.get('total_employees', 0)
        efficiency = market_cap / employees if employees > 0 else 0
        
        instances.append({
            'ticker': ticker,
            'timestamp': str(pd.Timestamp.now().date()),
            'company_name': company.get('name', ticker),
            'sic_code': company.get('sic_code', 'N/A'),
            'sic_description': company.get('sic_description', 'N/A'),
            'sector': company.get('sector', 'N/A'),
            'industry': company.get('industry', 'N/A'), 
            'market_cap': f"${market_cap/1e9:.1f}B" if market_cap > 0 else "N/A",
            'total_employees': f"{employees:,}" if employees > 0 else "N/A",
            'efficiency_metric': f"${efficiency:,.0f}" if efficiency > 0 else "N/A",
            'returns_10d': f"{returns_10d*100:.1f}%" if returns_10d != 0 else "N/A",
            'score': min(1.0, (efficiency / 1000000) + (returns_10d * 2)) if efficiency > 0 else 0.3,
            'message': f"{ticker} ({company.get('sic_code')}) - {company.get('sic_description', 'Unknown')}: {returns_10d*100:.1f}% (10d), ${efficiency:,.0f}/employee efficiency"
        })
    
    # Sort by efficiency metric
    instances.sort(key=lambda x: float(x['efficiency_metric'].replace('$', '').replace(',', '')) if x['efficiency_metric'] != "N/A" else 0, reverse=True)
    
    return instances
''' 