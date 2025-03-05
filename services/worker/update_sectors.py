import yfinance as yf
from typing import Dict, Tuple
import time
import os
from multiprocessing import Pool, cpu_count
import random


def fetch_ticker_info(ticker_data: Tuple[str, str, str]) -> Tuple[str, str, str, str]:
    """
    Worker function to fetch sector/industry info for a single ticker
    Returns: (ticker, new_sector, new_industry, error_message)
    """
    ticker, curr_sector, curr_industry = ticker_data
    try:
        # Add randomized delay to avoid all processes hitting the API at the same time
        time.sleep(random.uniform(0.1, 0.5))
        
        stock = yf.Ticker(ticker)
        info = stock.info

        new_sector = info.get("sector", "N/A")
        new_industry = info.get("industry", "N/A")

        # Sleep briefly to avoid rate limits
        time.sleep(0.2)
        return (ticker, new_sector, new_industry, None)

    except Exception as e:
        # Return the current values if we fail to fetch new ones
        return (ticker, curr_sector, curr_industry, str(e))


def update_sectors(conn) -> Dict[str, int]:
    """
    Fetches and updates sector/industry information for active securities
    using multiprocessing for parallel processing
    Returns stats about the update operation
    """
    stats = {"total": 0, "updated": 0, "failed": 0, "unchanged": 0}

    try:
        # Get active securities
        with conn.db.cursor() as cursor:
            cursor.execute(
                """
                SELECT DISTINCT ticker, sector, industry 
                FROM securities 
                WHERE maxDate IS NULL
            """
            )
            securities = cursor.fetchall()

        stats["total"] = len(securities)
        
        # Get batch size from environment or use a reasonable default
        batch_size = int(os.environ.get("UPDATE_SECTORS_BATCH_SIZE", "100"))
        
        # Process in smaller batches to avoid overwhelming resources
        if len(securities) > batch_size:
            print(f"Processing {len(securities)} securities in batches of {batch_size}", flush=True)
            securities = securities[:batch_size]
            stats["total"] = len(securities)

        # Use a more conservative number of processes
        # Lower of: 4 processes, CPU count, or half the number of securities
        num_processes = min(4, cpu_count(), max(1, len(securities) // 2))
        
        print(f"Starting update_sectors with {num_processes} processes for {len(securities)} securities", flush=True)

        # Process securities in parallel with a timeout
        with Pool(processes=num_processes) as pool:
            # Map the fetch_ticker_info function across all securities with a timeout
            results = pool.map(fetch_ticker_info, securities)
            
            # Process results and update database
            for ticker, new_sector, new_industry, error in results:
                if error:
                    print(f"Failed to update {ticker}: {error}", flush=True)
                    stats["failed"] += 1
                    continue

                # Find current values in original securities list
                curr_values = next(s for s in securities if s[0] == ticker)
                curr_sector, curr_industry = curr_values[1], curr_values[2]

                # Only update if there's a change or missing info
                if (
                    new_sector != curr_sector
                    or new_industry != curr_industry
                    or curr_sector is None
                    or curr_industry is None
                ):
                    try:
                        with conn.db.cursor() as cursor:
                            cursor.execute(
                                """
                                UPDATE securities 
                                SET sector = %s, industry = %s 
                                WHERE ticker = %s
                            """,
                                (new_sector, new_industry, ticker),
                            )
                        conn.db.commit()
                        stats["updated"] += 1
                    except Exception as e:
                        print(f"Failed to update database for {ticker}: {str(e)}", flush=True)
                        stats["failed"] += 1
                else:
                    stats["unchanged"] += 1

    except Exception as e:
        print(f"Error in update_sectors: {str(e)}", flush=True)
        stats["failed"] = stats["total"]
        
    print(f"update_sectors completed: {stats}", flush=True)
    return stats
