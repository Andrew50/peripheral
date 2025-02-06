import sys
from contextlib import contextmanager

@contextmanager
def pool_context(processes):
    """Safely handle process pool creation and cleanup"""
    pool = Pool(processes)
    try:
        yield pool
    finally:
        pool.close()
        pool.join()

def get_sector_info(ticker_symbol):
    try:
        # Force flush stdout for multiprocessing
        sys.stdout.flush()
        print(f"{get_timestamp()} - Fetching info for {ticker_symbol}", flush=True)
        sys.stdout.flush()
        
        ticker = yf.Ticker(ticker_symbol)
        info = ticker.info
        result = {
            'ticker': ticker_symbol,
            'sector': info.get('sector', 'Unknown'),
            'industry': info.get('industry', 'Unknown')
        }
        sys.stdout.flush()
        print(f"{get_timestamp()} - Successfully fetched {ticker_symbol}: {result}", flush=True)
        sys.stdout.flush()
        return result
    except Exception as e:
        sys.stdout.flush()
        print(f"{get_timestamp()} - Error fetching info for {ticker_symbol}: {str(e)}", flush=True)
        sys.stdout.flush()
        return {
            'ticker': ticker_symbol,
            'sector': 'Unknown',
            'industry': 'Unknown'
        }

def update_sectors(conn):
    print(f"{get_timestamp()} - Starting sector updates", flush=True)
    sys.stdout.flush()
    
    try:
        with conn.cursor() as cursor:
            # ... existing query code ...
            
            num_processes = min(cpu_count(), 3)
            batch_size = 20
            batches = [tickers[i:i + batch_size] for i in range(0, len(tickers), batch_size)]
            
            total_updates = 0
            with pool_context(num_processes) as pool:
                try:
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
                            print(f"{get_timestamp()} - Batch update successful: {len(updates)} records", flush=True)
                            sys.stdout.flush()
                            
                        except Exception as e:
                            print(f"{get_timestamp()} - Error in batch update: {str(e)}", flush=True)
                            sys.stdout.flush()
                            conn.rollback()
                            continue
                except Exception as e:
                    print(f"{get_timestamp()} - Error in pool processing: {str(e)}", flush=True)
                    sys.stdout.flush()
                    raise
            
            print(f"{get_timestamp()} - Completed! Successfully updated {total_updates} securities", flush=True)
            sys.stdout.flush()
                
    except Exception as e:
        print(f"{get_timestamp()} - Error in update_sectors: {str(e)}", flush=True)
        sys.stdout.flush()
        raise 