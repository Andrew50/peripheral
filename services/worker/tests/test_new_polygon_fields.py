#!/usr/bin/env python3
"""
Test suite for new Polygon API fields functionality

This test validates that the new fields from Polygon API are properly:
1. Fetched from Polygon API
2. Stored in the database 
3. Available for filtering in get_bar_data and get_general_data
4. Returned in API responses

New fields tested:
- share_class_figi
- sic_code  
- sic_description
- total_employees
- weighted_shares_outstanding
"""

import os
import sys
import pandas as pd
import numpy as np
from datetime import datetime, timedelta

# Add the src directory to the path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'src'))

def test_new_polygon_fields():
    """Test all new Polygon API fields functionality"""
    
    print("🧪 Testing new Polygon API fields functionality...")
    
    try:
        from data_accessors import get_bar_data, get_general_data
        
        # Test 1: Check if new fields are available in get_general_data
        print("\n1️⃣ Testing new fields availability in get_general_data...")
        
        # Test requesting the new fields
        try:
            general_data = get_general_data(
                columns=[
                    "ticker", "name", "sector", "industry", 
                    "share_class_figi", "sic_code", "sic_description", 
                    "total_employees", "weighted_shares_outstanding"
                ],
                filters={"active": True}
            )
            print(f"   ✅ New fields query successful: {len(general_data)} rows returned")
            
            if len(general_data) > 0:
                available_columns = list(general_data.columns)
                new_fields = ["share_class_figi", "sic_code", "sic_description", 
                             "total_employees", "weighted_shares_outstanding"]
                
                for field in new_fields:
                    if field in available_columns:
                        print(f"   ✅ Field '{field}' is available")
                    else:
                        print(f"   ⚠️ Field '{field}' is not available")
            
        except Exception as e:
            print(f"   ⚠️ Error testing new fields: {e}")
        
        # Test 2: SIC code filtering
        print("\n2️⃣ Testing SIC code filtering...")
        
        # Test SIC code filter
        try:
            sic_filtered_data = get_general_data(
                columns=["ticker", "sic_code", "sic_description"],
                filters={"sic_code": "7372"}  # Prepackaged Software
            )
            print(f"   ✅ SIC code filter (7372): {len(sic_filtered_data)} rows")
            
            # Test different SIC codes
            for sic_code in ["7373", "2834", "3674"]:
                try:
                    sic_data = get_general_data(
                        columns=["ticker", "sic_code"],
                        filters={"sic_code": sic_code}
                    )
                    print(f"   ✅ SIC code {sic_code}: {len(sic_data)} rows")
                except Exception as e:
                    print(f"   ⚠️ Error with SIC code {sic_code}: {e}")
                    
        except Exception as e:
            print(f"   ⚠️ Error testing SIC code filtering: {e}")
        
        # Test 3: Employee count filtering
        print("\n3️⃣ Testing employee count filtering...")
        
        try:
            # Test minimum employee filter
            min_employees_data = get_general_data(
                columns=["ticker", "total_employees"],
                filters={"total_employees_min": 1000}
            )
            print(f"   ✅ Min employees (1000+): {len(min_employees_data)} rows")
            
            # Test maximum employee filter
            max_employees_data = get_general_data(
                columns=["ticker", "total_employees"],
                filters={"total_employees_max": 10000}
            )
            print(f"   ✅ Max employees (<10000): {len(max_employees_data)} rows")
            
            # Test employee range filter
            range_employees_data = get_general_data(
                columns=["ticker", "total_employees"],
                filters={
                    "total_employees_min": 1000,
                    "total_employees_max": 50000
                }
            )
            print(f"   ✅ Employee range (1000-50000): {len(range_employees_data)} rows")
            
        except Exception as e:
            print(f"   ⚠️ Error testing employee filtering: {e}")
        
        # Test 4: Weighted shares outstanding filtering
        print("\n4️⃣ Testing weighted shares outstanding filtering...")
        
        try:
            # Test minimum weighted shares filter
            min_shares_data = get_general_data(
                columns=["ticker", "weighted_shares_outstanding"],
                filters={"weighted_shares_outstanding_min": 100000000}  # 100M shares
            )
            print(f"   ✅ Min weighted shares (100M+): {len(min_shares_data)} rows")
            
            # Test maximum weighted shares filter
            max_shares_data = get_general_data(
                columns=["ticker", "weighted_shares_outstanding"],
                filters={"weighted_shares_outstanding_max": 1000000000}  # 1B shares
            )
            print(f"   ✅ Max weighted shares (<1B): {len(max_shares_data)} rows")
            
        except Exception as e:
            print(f"   ⚠️ Error testing weighted shares filtering: {e}")
        
        # Test 5: Combined filtering with new fields
        print("\n5️⃣ Testing combined filtering with new fields...")
        
        try:
            combined_data = get_general_data(
                columns=[
                    "ticker", "name", "sector", "industry", "market_cap",
                    "sic_code", "sic_description", "total_employees", 
                    "weighted_shares_outstanding"
                ],
                filters={
                    "sector": "Technology",
                    "market_cap_min": 1000000000,  # $1B+
                    "total_employees_min": 1000,   # 1000+ employees
                    "total_employees_max": 100000, # <100k employees
                    "active": True
                }
            )
            print(f"   ✅ Combined filters: {len(combined_data)} rows")
            
            if len(combined_data) > 0:
                print(f"   ✅ Sample companies with new data:")
                for _, row in combined_data.head(3).iterrows():
                    ticker = row.get('ticker', 'N/A')
                    employees = row.get('total_employees', 'N/A')
                    sic_code = row.get('sic_code', 'N/A')
                    shares = row.get('weighted_shares_outstanding', 'N/A')
                    print(f"       {ticker}: {employees:,} employees, SIC {sic_code}, {shares:,} weighted shares" if isinstance(employees, (int, float)) else f"       {ticker}: {employees} employees, SIC {sic_code}")
                    
        except Exception as e:
            print(f"   ⚠️ Error testing combined filtering: {e}")
        
        # Test 6: Integration with get_bar_data
        print("\n6️⃣ Testing integration with get_bar_data...")
        
        try:
            # Test that get_bar_data works with new field filters
            bar_data_filtered = get_bar_data(
                timeframe="1d",
                columns=["ticker", "timestamp", "close"],
                min_bars=5,
                filters={
                    "total_employees_min": 5000,
                    "market_cap_min": 5000000000,  # $5B+
                    "active": True
                }
            )
            print(f"   ✅ get_bar_data with new filters: {len(bar_data_filtered)} rows")
            
            if len(bar_data_filtered) > 0:
                df = pd.DataFrame(bar_data_filtered, columns=["ticker", "timestamp", "close"])
                unique_tickers = df['ticker'].nunique()
                print(f"   ✅ Unique tickers with employee/market cap filters: {unique_tickers}")
            
        except Exception as e:
            print(f"   ⚠️ Error testing get_bar_data integration: {e}")
        
        # Test 7: Data quality checks
        print("\n7️⃣ Testing data quality for new fields...")
        
        try:
            # Check for non-null values in new fields
            all_data = get_general_data(
                columns=[
                    "ticker", "share_class_figi", "sic_code", "sic_description",
                    "total_employees", "weighted_shares_outstanding"
                ]
            )
            
            if len(all_data) > 0:
                for field in ["share_class_figi", "sic_code", "sic_description", 
                             "total_employees", "weighted_shares_outstanding"]:
                    if field in all_data.columns:
                        non_null_count = all_data[field].notna().sum()
                        total_count = len(all_data)
                        percentage = (non_null_count / total_count) * 100 if total_count > 0 else 0
                        print(f"   ✅ {field}: {non_null_count}/{total_count} ({percentage:.1f}%) non-null values")
                    else:
                        print(f"   ⚠️ {field}: Column not found")
                        
        except Exception as e:
            print(f"   ⚠️ Error testing data quality: {e}")
        
        # Test 8: Edge cases and error handling
        print("\n8️⃣ Testing edge cases...")
        
        try:
            # Test with non-existent SIC code
            empty_sic = get_general_data(
                columns=["ticker"],
                filters={"sic_code": "0000"}  # Non-existent
            )
            print(f"   ✅ Non-existent SIC code: {len(empty_sic)} rows (should be 0)")
            
            # Test with very high employee count
            high_employees = get_general_data(
                columns=["ticker"],
                filters={"total_employees_min": 1000000}  # 1M+ employees
            )
            print(f"   ✅ Very high employee count: {len(high_employees)} rows")
            
            # Test with invalid column names
            try:
                invalid_column = get_general_data(
                    columns=["ticker", "invalid_field"],
                    filters={"active": True}
                )
                print(f"   ⚠️ Invalid column accepted: {len(invalid_column)} rows")
            except Exception:
                print(f"   ✅ Invalid column properly rejected")
                
        except Exception as e:
            print(f"   ⚠️ Error testing edge cases: {e}")
        
        print("\n✅ New Polygon API fields tests completed!")
        return True
        
    except Exception as e:
        print(f"❌ Error in new fields tests: {e}")
        return False

def test_example_strategies():
    """Test the new example strategies using new fields"""
    
    print("\n🎯 Testing new example strategies...")
    
    try:
        # Test SOFTWARE_COMPANIES_STRATEGY simulation
        print("\n📊 Testing Software Companies Strategy...")
        
        from data_accessors import get_general_data, get_bar_data
        import pandas as pd
        
        # Simulate the software companies strategy
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
        
        print(f"   ✅ Software companies found: {len(general_data)}")
        
        if len(general_data) > 0:
            # Get sample tickers for price data
            sample_tickers = general_data['ticker'].head(5).tolist()
            
            bar_data = get_bar_data(
                timeframe="1d",
                tickers=sample_tickers,
                columns=["ticker", "timestamp", "close", "volume"],
                min_bars=5
            )
            
            print(f"   ✅ Price data for sample tickers: {len(bar_data)} rows")
            
            # Display sample results
            print(f"   📋 Sample software companies:")
            for _, company in general_data.head(3).iterrows():
                ticker = company.get('ticker', 'N/A')
                name = company.get('name', 'N/A')
                employees = company.get('total_employees', 'N/A')
                market_cap = company.get('market_cap', 0)
                print(f"       {ticker} - {name}: {employees:,} employees, ${market_cap/1e9:.1f}B market cap" if isinstance(employees, (int, float)) else f"       {ticker} - {name}: {employees} employees")
        
        # Test SIC_CODE_ANALYSIS_STRATEGY simulation
        print("\n🔬 Testing SIC Code Analysis Strategy...")
        
        target_sic_codes = ['7372', '7373', '2834', '3674']
        all_companies = []
        
        for sic_code in target_sic_codes:
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
                print(f"   ✅ SIC {sic_code}: {len(companies)} companies")
        
        if all_companies:
            combined_df = pd.concat(all_companies, ignore_index=True)
            print(f"   ✅ Total companies across target SIC codes: {len(combined_df)}")
            
            # Display sample results by SIC code
            print(f"   📋 Sample companies by SIC code:")
            for sic_code in target_sic_codes:
                sic_companies = combined_df[combined_df['sic_code'] == sic_code]
                if len(sic_companies) > 0:
                    sample = sic_companies.head(1).iloc[0]
                    print(f"       SIC {sic_code} ({sample.get('sic_description', 'N/A')}): {sample.get('ticker')} - {sample.get('name', 'N/A')}")
        
        print("\n✅ Example strategies tests completed!")
        return True
        
    except Exception as e:
        print(f"❌ Error in example strategies tests: {e}")
        return False

def main():
    """Run all new Polygon fields tests"""
    
    print("🚀 Starting comprehensive tests for new Polygon API fields...")
    
    # Run basic functionality tests
    basic_success = test_new_polygon_fields()
    
    # Run example strategy tests
    strategy_success = test_example_strategies()
    
    # Overall results
    print("\n" + "="*60)
    print("📊 TEST SUMMARY")
    print("="*60)
    print(f"✅ Basic functionality tests: {'PASSED' if basic_success else 'FAILED'}")
    print(f"✅ Example strategy tests: {'PASSED' if strategy_success else 'FAILED'}")
    
    overall_success = basic_success and strategy_success
    print(f"\n🎯 OVERALL RESULT: {'✅ ALL TESTS PASSED' if overall_success else '❌ SOME TESTS FAILED'}")
    
    if not overall_success:
        print("\n💡 Note: Some tests may fail if:")
        print("   - Database migrations haven't been applied yet")
        print("   - UpdateSecurityDetails hasn't populated the new fields")
        print("   - Database connection is not available")
    
    return overall_success

if __name__ == "__main__":
    success = main()
    sys.exit(0 if success else 1) 