import yfinance as yf
import psycopg2
from psycopg2.extras import execute_batch
import time
from multiprocessing import Pool, cpu_count
from datetime import datetime

def get_timestamp():
    return datetime.now().strftime('%Y-%m-%d %H:%M:%S')

def get_sector_info(ticker_symbol):
    try:
        ticker = yf.Ticker(ticker_symbol)
        info = ticker.info
        return {
            'ticker': ticker_symbol,
            'sector': info.get('sector', 'Unknown'),
            'industry': info.get('industry', 'Unknown')
        }
    except Exception as e:
        print(f"{get_timestamp()} - Error fetching info for {ticker_symbol}: {str(e)}", flush=True)
        return {
            'ticker': ticker_symbol,
            'sector': 'Unknown',
            'industry': 'Unknown'
        }

def process_ticker_batch(tickers):
    results = []
    for ticker in tickers:
        info = get_sector_info(ticker)
        results.append(info)
        time.sleep(0.5)  # Respect rate limits while still being parallel
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
                return
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
                        
                    except Exception as e:
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
