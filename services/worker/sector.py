import yfinance as yf
import psycopg2
from psycopg2.extras import execute_batch
import time
import random
from conn import Conn
from multiprocessing import Pool, cpu_count
from datetime import datetime

USE_DATABASE = True  # Set to False to print output instead of saving to database

# Rate limiting parameters
INITIAL_SLEEP_TIME = 0.5  # Initial sleep time between requests
MAX_SLEEP_TIME = 30  # Maximum sleep time after backoff
BACKOFF_FACTOR = 2  # Exponential backoff multiplier
JITTER = 0.1  # Random jitter factor to avoid synchronized requests

def get_timestamp():
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def get_sector_info(ticker_symbol, retry_count=0, max_retries=3):
    try:
        ticker = yf.Ticker(ticker_symbol)
        info = ticker.info
        
        # Check if we got meaningful data
        sector = info.get("sector", "Unknown")
        industry = info.get("industry", "Unknown")
        
        print(f"{get_timestamp()} - Successfully fetched info for {ticker_symbol}: sector={sector}, industry={industry}", flush=True)
        
        return {
            "ticker": ticker_symbol,
            "sector": sector,
            "industry": industry,
            "status": "success"
        }
    except Exception as e:
        error_msg = str(e)
        print(
            f"{get_timestamp()} - Error fetching info for {ticker_symbol}: {error_msg}",
            flush=True,
        )
        
        # Handle rate limiting with exponential backoff
        if "Too Many Requests" in error_msg and retry_count < max_retries:
            retry_count += 1
            sleep_time = min(INITIAL_SLEEP_TIME * (BACKOFF_FACTOR ** retry_count), MAX_SLEEP_TIME)
            # Add jitter to avoid synchronized requests
            sleep_time = sleep_time * (1 + random.uniform(-JITTER, JITTER))
            
            print(f"{get_timestamp()} - Rate limited for {ticker_symbol}. Retrying in {sleep_time:.2f}s (attempt {retry_count}/{max_retries})", flush=True)
            time.sleep(sleep_time)
            return get_sector_info(ticker_symbol, retry_count, max_retries)
            
        return {
            "ticker": ticker_symbol, 
            "sector": "Unknown", 
            "industry": "Unknown",
            "status": "failed",
            "error": error_msg
        }


def process_ticker_batch(tickers):
    results = []
    success_count = 0
    failure_count = 0
    
    print(f"{get_timestamp()} - Processing batch of {len(tickers)} tickers", flush=True)
    
    for ticker in tickers:
        info = get_sector_info(ticker)
        results.append(info)
        
        if info.get("status") == "success":
            success_count += 1
        else:
            failure_count += 1
            
        # Dynamic sleep time based on recent success rate
        current_success_rate = success_count / (success_count + failure_count) if (success_count + failure_count) > 0 else 0.5
        # Adjust sleep time - sleep longer if we're getting more failures
        sleep_time = INITIAL_SLEEP_TIME * (1 + (1 - current_success_rate) * 2)
        sleep_time = sleep_time * (1 + random.uniform(-JITTER, JITTER))  # Add jitter
        
        time.sleep(sleep_time)
        
    print(f"{get_timestamp()} - Batch completed: {success_count} successful, {failure_count} failed", flush=True)
    return results


def update_sectors(conn):
    start_time = datetime.now()
    print(f"{get_timestamp()} - Starting sector updates", flush=True)
    stats = {"total": 0, "updated": 0, "failed": 0, "unchanged": 0}
    
    try:
        with conn.db.cursor() as cursor:
            cursor.execute(
                """
                SELECT DISTINCT ticker 
                FROM securities 
                WHERE maxDate IS NULL
            """
            )
            all_tickers = [row[0] for row in cursor.fetchall()]
            if not all_tickers:
                print(f"{get_timestamp()} - No tickers found to update", flush=True)
                return stats

            total_tickers = len(all_tickers)
            stats["total"] = total_tickers
            print(f"{get_timestamp()} - Processing {total_tickers} securities in batches of 100", flush=True)
            
            # Process in batches of 100
            batch_size = 100
            for i in range(0, total_tickers, batch_size):
                batch = all_tickers[i:i+batch_size]
                batch_start_time = datetime.now()
                print(f"{get_timestamp()} - Starting update_sectors with {min(cpu_count(), 4)} processes for {len(batch)} securities", flush=True)
                
                # Split the batch for parallel processing
                num_processes = min(cpu_count(), 4)  # Use at most 4 processes to avoid overwhelming the API
                chunks = [batch[j::num_processes] for j in range(num_processes)]
                
                # Process in parallel
                with Pool(num_processes) as pool:
                    all_results = pool.map(process_ticker_batch, chunks)
                
                # Flatten results
                batch_results = [item for sublist in all_results for item in sublist]
                
                # Update database with results
                batch_updated = 0
                batch_failed = 0
                batch_unchanged = 0
                
                for info in batch_results:
                    if USE_DATABASE:
                        try:
                            if info.get("status") == "success":
                                cursor.execute(
                                    """
                                    UPDATE securities 
                                    SET sector = %s, industry = %s 
                                    WHERE ticker = %s AND maxDate IS NULL
                                    RETURNING (xmax = 0) AS was_inserted
                                """,
                                    (info["sector"], info["industry"], info["ticker"]),
                                )
                                
                                result = cursor.fetchone()
                                if result and cursor.rowcount > 0:
                                    batch_updated += 1
                                else:
                                    batch_unchanged += 1
                                    
                                conn.db.commit()
                            else:
                                batch_failed += 1
                                print(f"{get_timestamp()} - Failed to update {info['ticker']}: {info.get('error', 'Unknown error')}", flush=True)

                        except Exception as e:
                            conn.db.rollback()
                            batch_failed += 1
                            print(
                                f"{get_timestamp()} - Database error updating {info['ticker']}: {str(e)}",
                                flush=True,
                            )
                
                # Update stats
                stats["updated"] += batch_updated
                stats["failed"] += batch_failed
                stats["unchanged"] += batch_unchanged
                
                batch_end_time = datetime.now()
                batch_duration = batch_end_time - batch_start_time
                
                print(
                    f"{get_timestamp()} - Batch {i//batch_size + 1}/{(total_tickers+batch_size-1)//batch_size} completed: "
                    f"{batch_updated} updated, {batch_failed} failed, {batch_unchanged} unchanged. "
                    f"Time: {batch_duration}",
                    flush=True,
                )
                
                # Sleep between batches to avoid overwhelming the API
                time.sleep(2)

            end_time = datetime.now()
            duration = end_time - start_time
            print(
                f"{get_timestamp()} - update_sectors completed: {stats}",
                flush=True,
            )
            print(f"finished update_sectors {{}} time: {duration}", flush=True)
            return stats

    except Exception as e:
        print(f"{get_timestamp()} - Error in update_sectors: {str(e)}", flush=True)
        raise


if __name__ == "__main__":
    # Test code
    update_sectors(Conn(False))

