import yfinance as yf
import requests
from conn import Conn, set_task_context, log_message, get_timestamp, add_task_log
from multiprocessing import Pool, cpu_count
from datetime import datetime, timedelta
import time
import random
import json
import sys
import traceback

USE_DATABASE = True  # Set to False to print output instead of saving to database

# Global current task data and ID - No longer needed, moved to conn.py
# CURRENT_TASK_DATA = None
# CURRENT_TASK_ID = None

# Rate limiting parameters - More conservative values
INITIAL_SLEEP_TIME = 2.0  # Increased from 1.0 to 2.0 seconds
MAX_SLEEP_TIME = 120  # Increased from 60 to 120 seconds
BACKOFF_FACTOR = 3  # Increased from 2 to 3 for more aggressive backoff
JITTER = 0.2  # Increased jitter to spread requests
BATCH_SIZE = 10  # Dramatically reduced batch size from 20 to 5
MAX_PARALLEL_PROCESSES = 1  # Reduced from 2 to 1 to avoid parallel requests

# Global rate limit pause threshold
RATE_LIMIT_PAUSE_THRESHOLD = 0.5  # Pause processing if 50% of requests are rate limited
GLOBAL_PAUSE_TIME = 300  # 5 minutes pause when threshold is exceeded

# Additional tracking for rate limiting
RATE_LIMIT_STATS = {
    "total_requests": 0,
    "rate_limited_requests": 0,
    "consecutive_rate_limits": 0,
    "max_consecutive_rate_limits": 0,
    "backoff_activations": 0,
    "largest_backoff": 0,
    "batches_with_rate_limits": 0,
    "total_batches": 0,
    "rate_limit_start_time": None
}

def log_rate_limit_stats(prefix=""):
    """Log current rate limiting statistics"""
    current_rate = RATE_LIMIT_STATS["rate_limited_requests"] / max(RATE_LIMIT_STATS["total_requests"], 1) * 100
    
    rate_limit_duration = ""
    if RATE_LIMIT_STATS["rate_limit_start_time"]:
        duration = datetime.now() - RATE_LIMIT_STATS["rate_limit_start_time"]
        rate_limit_duration = f", duration: {duration}"
    
    log_message(f"{get_timestamp()} - {prefix} RATE LIMIT STATS: "
          f"{RATE_LIMIT_STATS['rate_limited_requests']}/{RATE_LIMIT_STATS['total_requests']} "
          f"requests rate limited ({current_rate:.1f}%), "
          f"consecutive: {RATE_LIMIT_STATS['consecutive_rate_limits']}, "
          f"max consecutive: {RATE_LIMIT_STATS['max_consecutive_rate_limits']}, "
          f"backoffs: {RATE_LIMIT_STATS['backoff_activations']}, "
          f"max backoff: {RATE_LIMIT_STATS['largest_backoff']:.1f}s, "
          f"batches: {RATE_LIMIT_STATS['batches_with_rate_limits']}/{RATE_LIMIT_STATS['total_batches']}"
          f"{rate_limit_duration}")

def get_sector_info(ticker_symbol, retry_count=0, max_retries=5):  # Increased max_retries from 3 to 5
    """Get sector and industry information for a ticker symbol"""
    try:
        # Increment request counter
        RATE_LIMIT_STATS["total_requests"] += 1
        
        # Get ticker info with exponential backoff on failure
        ticker = yf.Ticker(ticker_symbol)
        info = ticker.info
        
        sector = info.get("sector", "Unknown")
        industry = info.get("industry", "Unknown")
        
        log_message(f"{get_timestamp()} - Successfully fetched info for {ticker_symbol}: sector={sector}, industry={industry}")
        
        # Reset consecutive rate limit counter on success
        RATE_LIMIT_STATS["consecutive_rate_limits"] = 0
        
        return {
            "ticker": ticker_symbol,
            "sector": sector,
            "industry": industry,
            "status": "success"
        }
    except Exception as e:
        error_msg = str(e)
        
        # Enhanced error handling with more detailed diagnostics
        if "Too Many Requests" in error_msg:
            log_message(str(e), "warn")
            log_message(f"{get_timestamp()} - Failed to update {ticker_symbol}: Too Many Requests. Rate limited. Try after a while.", "warn")
            
            # Track rate limit statistics
            RATE_LIMIT_STATS["rate_limited_requests"] += 1
            RATE_LIMIT_STATS["consecutive_rate_limits"] += 1
            
            # Update max consecutive counter
            if RATE_LIMIT_STATS["consecutive_rate_limits"] > RATE_LIMIT_STATS["max_consecutive_rate_limits"]:
                RATE_LIMIT_STATS["max_consecutive_rate_limits"] = RATE_LIMIT_STATS["consecutive_rate_limits"]
            
            # Set rate limit start time if this is the first one
            if RATE_LIMIT_STATS["rate_limit_start_time"] is None:
                RATE_LIMIT_STATS["rate_limit_start_time"] = datetime.now()
            
            # Enhanced retry mechanism with exponential backoff
            if retry_count < max_retries:
                retry_count += 1
                RATE_LIMIT_STATS["backoff_activations"] += 1
                
                # Calculate exponential backoff with added penalty for high consecutive rate limits
                consecutive_factor = min(RATE_LIMIT_STATS["consecutive_rate_limits"], 10) / 2  # Up to 5x multiplier
                sleep_time = INITIAL_SLEEP_TIME * (BACKOFF_FACTOR ** retry_count) * (1 + consecutive_factor)
                
                # Cap at MAX_SLEEP_TIME
                sleep_time = min(sleep_time, MAX_SLEEP_TIME)
                
                # Add jitter to avoid synchronized requests
                sleep_time = sleep_time * (1 + random.uniform(-JITTER, JITTER))
                
                # Track largest backoff
                if sleep_time > RATE_LIMIT_STATS["largest_backoff"]:
                    RATE_LIMIT_STATS["largest_backoff"] = sleep_time
                
                print(f"{get_timestamp()} - Rate limited for {ticker_symbol}. Retrying in {sleep_time:.2f}s (attempt {retry_count}/{max_retries}, consecutive: {RATE_LIMIT_STATS['consecutive_rate_limits']})", flush=True)
                
                # Log rate limit stats periodically
                if RATE_LIMIT_STATS["rate_limited_requests"] % 5 == 0:
                    log_rate_limit_stats()
                
                time.sleep(sleep_time)
                return get_sector_info(ticker_symbol, retry_count, max_retries)
            else:
                detailed_error = "Yahoo Finance API rate limit exceeded. Exhausted retry attempts."
        else:
            print(f"{get_timestamp()} - Error fetching info for {ticker_symbol}: {error_msg}", flush=True)
            detailed_error = error_msg
            
        return {
            "ticker": ticker_symbol, 
            "sector": "Unknown", 
            "industry": "Unknown",
            "status": "failed",
            "error": detailed_error
        }


def process_ticker_batch(tickers):
    results = []
    success_count = 0
    failure_count = 0
    rate_limit_count = 0
    
    print(f"{get_timestamp()} - Processing batch of {len(tickers)} tickers", flush=True)
    
    for ticker in tickers:
        # Check if we need to pause due to excessive rate limiting
        if RATE_LIMIT_STATS["total_requests"] > 10:  # Only check after we have enough data
            rate_limit_ratio = RATE_LIMIT_STATS["rate_limited_requests"] / RATE_LIMIT_STATS["total_requests"]
            if rate_limit_ratio >= RATE_LIMIT_PAUSE_THRESHOLD:
                pause_time = GLOBAL_PAUSE_TIME * (1 + random.uniform(-JITTER, JITTER))
                print(f"{get_timestamp()} - EXCESSIVE RATE LIMITING DETECTED ({rate_limit_ratio:.1%} of requests). Pausing for {pause_time:.1f} seconds before continuing", flush=True)
                log_rate_limit_stats("PAUSING -")
                time.sleep(pause_time)
                # Reset stats after pause
                RATE_LIMIT_STATS["consecutive_rate_limits"] = 0
        
        info = get_sector_info(ticker)
        results.append(info)
        
        if info.get("status") == "success":
            success_count += 1
        else:
            failure_count += 1
            # Count rate limit errors specifically
            if "rate limit" in info.get('error', '').lower():
                rate_limit_count += 1
            
        # Dynamic sleep time based on recent success rate and rate limit trends
        current_success_rate = success_count / (success_count + failure_count) if (success_count + failure_count) > 0 else 0.5
        
        # Calculate base sleep time - more aggressive as success rate decreases
        base_sleep = INITIAL_SLEEP_TIME * (1 + (1 - current_success_rate) * 8)  # Increased multiplier from 4 to 8
        
        # Add rate limit scaling factor
        rate_limit_factor = 1.0
        if RATE_LIMIT_STATS["total_requests"] > 0:
            # Scale by overall rate limit ratio
            rate_limit_ratio = RATE_LIMIT_STATS["rate_limited_requests"] / RATE_LIMIT_STATS["total_requests"]
            rate_limit_factor = 1.0 + (rate_limit_ratio * 10)  # Up to 11x multiplier at 100% rate limiting
        
        sleep_time = base_sleep * rate_limit_factor
        
        # Add jitter to avoid synchronized requests
        sleep_time = sleep_time * (1 + random.uniform(-JITTER, JITTER))
        
        # Add additional sleep if we've had consecutive failures
        if failure_count > 2 and success_count == 0:  # Reduced threshold from 3 to 2
            sleep_time *= 3  # Triple sleep time (up from double)
            print(f"{get_timestamp()} - Consecutive failures detected. Tripling sleep time to {sleep_time:.2f}s", flush=True)
            
        # Cap at MAX_SLEEP_TIME
        sleep_time = min(sleep_time, MAX_SLEEP_TIME)
            
        # Log sleep time periodically
        if (success_count + failure_count) % 5 == 0:
            print(f"{get_timestamp()} - Current wait between requests: {sleep_time:.2f}s (success rate: {current_success_rate:.2f}, rate limit factor: {rate_limit_factor:.2f})", flush=True)
            
        time.sleep(sleep_time)
    
    # Log summary with rate limit information
    print(f"{get_timestamp()} - Batch completed: {success_count} successful, {failure_count} failed ({rate_limit_count} rate limited)", flush=True)
    
    # Update global stats
    RATE_LIMIT_STATS["total_batches"] += 1
    if rate_limit_count > 0:
        RATE_LIMIT_STATS["batches_with_rate_limits"] += 1
    
    return results


def update_sectors(conn):
    global RATE_LIMIT_STATS
    
    # Get task context from conn object if available
    if hasattr(conn, 'task_data') and hasattr(conn, 'task_id'):
        set_task_context(conn.task_data, conn.task_id)
    
    # Reset rate limit stats
    RATE_LIMIT_STATS = {
        "total_requests": 0,
        "rate_limited_requests": 0,
        "consecutive_rate_limits": 0,
        "max_consecutive_rate_limits": 0,
        "backoff_activations": 0,
        "largest_backoff": 0,
        "batches_with_rate_limits": 0,
        "total_batches": 0,
        "rate_limit_start_time": None
    }
    
    start_time = datetime.now()
    log_message(f"{get_timestamp()} - Starting sector updates")
    stats = {"total": 0, "updated": 0, "failed": 0, "unchanged": 0, "rate_limited": 0}
    
    try:
        with conn.db.cursor() as cursor:
            cursor.execute(
                """
                SELECT DISTINCT ticker 
                FROM securities 
                WHERE maxDate IS NULL
                LIMIT 100  -- Limit to 100 tickers per run to avoid excessive rate limiting
            """
            )
            all_tickers = [row[0] for row in cursor.fetchall()]
            if not all_tickers:
                log_message(f"{get_timestamp()} - No tickers found to update")
                return stats

            total_tickers = len(all_tickers)
            stats["total"] = total_tickers
            log_message(f"{get_timestamp()} - Processing {total_tickers} securities in batches of {BATCH_SIZE}")
            
            # Process in batches of BATCH_SIZE
            for i in range(0, total_tickers, BATCH_SIZE):
                # Check if we've hit severe rate limiting and should completely abort this run
                if RATE_LIMIT_STATS["total_requests"] > 20 and RATE_LIMIT_STATS["rate_limited_requests"] / RATE_LIMIT_STATS["total_requests"] > 0.8:
                    remaining_tickers = total_tickers - i
                    print(f"{get_timestamp()} - CRITICAL: Aborting sector updates due to severe rate limiting. {remaining_tickers} tickers left unprocessed.", flush=True)
                    log_rate_limit_stats("ABORTING -")
                    stats["failed"] += remaining_tickers
                    break
                
                batch = all_tickers[i:i+BATCH_SIZE]
                batch_start_time = datetime.now()
                print(f"{get_timestamp()} - Starting batch {i//BATCH_SIZE + 1}/{(total_tickers+BATCH_SIZE-1)//BATCH_SIZE} with {len(batch)} securities", flush=True)
                
                # Process sequentially or with limited parallelism based on our rate limit experience
                if MAX_PARALLEL_PROCESSES <= 1:
                    # Process sequentially
                    batch_results = process_ticker_batch(batch)
                else:
                    # Process with limited parallelism
                    num_processes = min(MAX_PARALLEL_PROCESSES, len(batch))
                    chunks = [batch[j::num_processes] for j in range(num_processes)]
                    
                    with Pool(num_processes) as pool:
                        all_results = pool.map(process_ticker_batch, chunks)
                    
                    # Flatten results
                    batch_results = [item for sublist in all_results for item in sublist]
                
                # Update database with results
                batch_updated = 0
                batch_failed = 0
                batch_unchanged = 0
                batch_rate_limited = 0
                
                RATE_LIMIT_STATS["total_batches"] += 1
                
                for info in batch_results:
                    if "rate limit" in info.get('error', '').lower():
                        batch_rate_limited += 1
                        
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
                                print(f"Failed to update {info['ticker']}: {info.get('error', 'Unknown error')}", flush=True)

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
                stats["rate_limited"] += batch_rate_limited
                
                # Check if we hit rate limits in this batch
                if batch_rate_limited > 0:
                    RATE_LIMIT_STATS["batches_with_rate_limits"] += 1
                
                batch_end_time = datetime.now()
                batch_duration = batch_end_time - batch_start_time
                
                print(
                    f"{get_timestamp()} - Batch {i//BATCH_SIZE + 1}/{(total_tickers+BATCH_SIZE-1)//BATCH_SIZE} completed: "
                    f"{batch_updated} updated, {batch_failed} failed ({batch_rate_limited} rate limited), {batch_unchanged} unchanged. "
                    f"Time: {batch_duration}",
                    flush=True,
                )
                
                # If all requests in this batch failed due to rate limiting, add a longer pause
                rate_limit_percentage = batch_rate_limited / len(batch) if len(batch) > 0 else 0
                
                if batch_rate_limited > 0:
                    # Log rate limit stats after each batch with rate limits
                    log_rate_limit_stats(f"Batch {i//BATCH_SIZE + 1} -")
                
                if rate_limit_percentage > 0.5:  # If more than 50% of requests were rate limited
                    # Calculate adaptive backoff based on rate limit percentage
                    adaptive_sleep = 60 + int(rate_limit_percentage * 180)  # 60-240 seconds based on severity
                    
                    print(f"{get_timestamp()} - High rate limiting detected ({batch_rate_limited}/{len(batch)} = {rate_limit_percentage:.1%}). "
                          f"Implementing adaptive backoff of {adaptive_sleep} seconds before next batch.", flush=True)
                    
                    time.sleep(adaptive_sleep)
                elif batch_failed == len(batch) and any("rate limit" in info.get('error', '').lower() for info in batch_results):
                    extended_sleep = 240  # Increased from 120 to 240 seconds for complete batch failures
                    print(f"{get_timestamp()} - All requests failed due to rate limiting. Pausing for {extended_sleep} seconds before next batch.", flush=True)
                    time.sleep(extended_sleep)
                else:
                    # Regular sleep between batches - increasing delay proportionally to rate limit ratio
                    if i + BATCH_SIZE < total_tickers:  # If there are more batches to process
                        rate_limit_ratio = RATE_LIMIT_STATS["rate_limited_requests"] / max(RATE_LIMIT_STATS["total_requests"], 1)
                        batch_interval = INITIAL_SLEEP_TIME * 5 * (1 + rate_limit_ratio * 5)  # Up to 30x initial sleep time
                        batch_interval = min(batch_interval, MAX_SLEEP_TIME)  # Cap at max sleep time
                        
                        print(f"{get_timestamp()} - Sleeping {batch_interval:.2f}s before next batch (rate limit ratio: {rate_limit_ratio:.2f})", flush=True)
                        time.sleep(batch_interval)
            
            end_time = datetime.now()
            duration = end_time - start_time
            
            # Log final rate limit statistics
            log_rate_limit_stats("FINAL -")
            
            # Enhanced completion logging with rate limit information
            print(
                f"{get_timestamp()} - update_sectors completed: "
                f"total={stats['total']}, updated={stats['updated']}, "
                f"failed={stats['failed']} (rate_limited={stats['rate_limited']}), "
                f"unchanged={stats['unchanged']}",
                flush=True,
            )
            
            # Add recommendations based on rate limiting
            if stats['rate_limited'] > 0:
                rate_limit_percentage = stats['rate_limited'] / stats['total'] if stats['total'] > 0 else 0
                if rate_limit_percentage > 0.8:
                    print(f"{get_timestamp()} - RECOMMENDATION: Severe rate limiting detected ({rate_limit_percentage:.1%} of requests). "
                          f"Consider implementing at least a 6-hour pause before retrying.", flush=True)
                elif rate_limit_percentage > 0.5:
                    print(f"{get_timestamp()} - RECOMMENDATION: Significant rate limiting ({rate_limit_percentage:.1%} of requests). "
                          f"Consider implementing a 3-hour pause before retrying.", flush=True)
                elif rate_limit_percentage > 0.2:
                    print(f"{get_timestamp()} - RECOMMENDATION: Moderate rate limiting ({rate_limit_percentage:.1%} of requests). "
                          f"Consider reducing batch size further or implementing a 1-hour pause.", flush=True)
            
            print(f"{get_timestamp()} - Total execution time: {duration}", flush=True)
            return stats

    except Exception as e:
        print(f"{get_timestamp()} - Error in update_sectors: {str(e)}", flush=True)
        # Log rate limit stats even on error
        if RATE_LIMIT_STATS["rate_limited_requests"] > 0:
            log_rate_limit_stats("ERROR")
        raise


if __name__ == "__main__":
    # Test code
    update_sectors(Conn(False))

