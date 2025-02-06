import yfinance as yf
import psycopg2
from psycopg2.extras import execute_batch
import time
from conn import Conn
from multiprocessing import Pool, cpu_count
from datetime import datetime

USE_DATABASE = True  # Set to False to print output instead of saving to database

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
        time.sleep(0.1)  # Respect rate limits while still being parallel
    return results

def update_sectors(conn):
    print(f"{get_timestamp()} - Starting sector updates", flush=True)
    try:
        with conn.db.cursor() as cursor:
            cursor.execute("""
                SELECT DISTINCT ticker 
                FROM securities 
                WHERE maxDate IS NULL
            """)
            tickers = [row[0] for row in cursor.fetchall()]
            if not tickers:
                return
            
            total_updates = 0
            total_tickers = len(tickers)
            print(f"{get_timestamp()} - Processing {total_tickers} tickers", flush=True)
            
            for ticker in tickers:
                info = get_sector_info(ticker)
                
                if USE_DATABASE:
                    try:
                        cursor.execute("""
                            UPDATE securities 
                            SET sector = %s, industry = %s 
                            WHERE ticker = %s AND maxDate IS NULL
                        """, (info['sector'], info['industry'], info['ticker']))
                        conn.db.commit()
                        total_updates += 1
                        
                        # Add progress tracking
                        if total_updates % 10 == 0:
                            print(f"{get_timestamp()} - Processed {total_updates}/{total_tickers} tickers ({(total_updates/total_tickers)*100:.1f}%)", flush=True)
                        
                    except Exception as e:
                        conn.db.rollback()
                        print(f"{get_timestamp()} - Error updating {ticker}: {str(e)}", flush=True)
                        continue
                
                time.sleep(0.3)  # Respect rate limits, .3 seems to be the min wait
            
            if USE_DATABASE:
                print(f"{get_timestamp()} - Completed! Successfully updated {total_updates} securities", flush=True)
                
    except Exception as e:
        print(f"{get_timestamp()} - Error in update_sectors: {str(e)}", flush=True)
        raise

if __name__ == "__main__":
    # Test code
    update_sectors(Conn(False))
