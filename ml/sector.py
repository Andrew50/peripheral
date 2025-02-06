import yfinance as yf
import psycopg2
from psycopg2.extras import execute_batch
import time
from multiprocessing import Pool, cpu_count
from datetime import datetime
import os

# Force stdout to flush immediately
os.environ['PYTHONUNBUFFERED'] = '1'

def get_timestamp():
    return datetime.now().strftime('%Y-%m-%d %H:%M:%S')

def get_sector_info(ticker_symbol):
    try:
        print(f"{get_timestamp()} - Processing ticker: {ticker_symbol}", flush=True)
        ticker = yf.Ticker(ticker_symbol)
        info = ticker.info
        result = {
            'ticker': ticker_symbol,
            'sector': info.get('sector', 'Unknown'),
            'industry': info.get('industry', 'Unknown')
        }
        print(f"{get_timestamp()} - Got data for {ticker_symbol}: {result['sector']}/{result['industry']}", flush=True)
        return result
    except Exception as e:
        print(f"{get_timestamp()} - Error fetching info for {ticker_symbol}: {str(e)}", flush=True)
        return {
            'ticker': ticker_symbol,
            'sector': 'Unknown',
            'industry': 'Unknown'
        }

def process_ticker_batch(tickers):
    print(f"{get_timestamp()} - Starting batch of {len(tickers)} tickers", flush=True)
    results = []
    for ticker in tickers:
        info = get_sector_info(ticker)
        results.append(info)
        time.sleep(0.5)  # Respect rate limits while still being parallel
    print(f"{get_timestamp()} - Completed batch of {len(tickers)} tickers", flush=True)
    return results

def update_sectors(conn):
    print(f"{get_timestamp()} - Starting sector updates", flush=True)
    try:
        with conn.cursor() as cursor:
            cursor.execute("""
                SELECT DISTINCT ticker 
                FROM securities 
                WHERE maxDate IS NULL
            """)
            tickers = [row[0] for row in cursor.fetchall()]
            
            if not tickers:
                print(f"{get_timestamp()} - No tickers found to update", flush=True)
                return
                
            print(f"{get_timestamp()} - Found {len(tickers)} tickers to update", flush=True)
            
            num_processes = min(cpu_count(), 3)
            batch_size = 20
            batches = [tickers[i:i + batch_size] for i in range(0, len(tickers), batch_size)]
            
            total_updates = 0
            with Pool(num_processes) as pool:
                for batch_results in pool.imap_unordered(process_ticker_batch, batches):
                    updates = [(info['sector'], info['industry'], info['ticker']) 
                              for info in batch_results]
                    
                    try:
                        execute_batch(cursor, """
                            UPDATE securities 
                            SET sector = %s, industry = %s 
                            WHERE ticker = %s AND maxDate IS NULL
                        """, updates)
                        conn.commit()
                        total_updates += len(updates)
                        print(f"{get_timestamp()} - Updated batch of {len(updates)} records. Total: {total_updates}", flush=True)
                        
                    except Exception as e:
                        print(f"{get_timestamp()} - Error updating batch: {str(e)}", flush=True)
                        conn.rollback()
                        continue
            
            print(f"{get_timestamp()} - Completed! Successfully updated {total_updates} securities", flush=True)
                
    except Exception as e:
        print(f"{get_timestamp()} - Error in update_sectors: {str(e)}", flush=True)
        raise

if __name__ == "__main__":
    # Test code
    ticker = "AAPL"
    info = get_sector_info(ticker)
    print(f"Sector: {info['sector']}", flush=True)
    print(f"Industry: {info['industry']}", flush=True) 