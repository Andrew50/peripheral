import yfinance as yf
from typing import List, Dict, Tuple
import time
from multiprocessing import Pool, cpu_count
from functools import partial


def fetch_ticker_info(ticker_data: Tuple[str, str, str]) -> Tuple[str, str, str, str]:
    """
    Worker function to fetch sector/industry info for a single ticker
    Returns: (ticker, new_sector, new_industry, error_message)
    """
    ticker, curr_sector, curr_industry = ticker_data
    try:
        stock = yf.Ticker(ticker)
        info = stock.info

        new_sector = info.get("sector", "N/A")
        new_industry = info.get("industry", "N/A")

        # Sleep briefly to avoid rate limits
        time.sleep(0.1)
        return (ticker, new_sector, new_industry, None)

    except Exception as e:
        return (ticker, None, None, str(e))


def update_sectors(conn) -> Dict[str, int]:
    """
    Fetches and updates sector/industry information for active securities
    using multiprocessing for parallel processing
    Returns stats about the update operation
    """
    stats = {"total": 0, "updated": 0, "failed": 0, "unchanged": 0}

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

    # Use max of 8 processes or CPU count
    num_processes = min(8, cpu_count())

    # Process securities in parallel
    with Pool(processes=num_processes) as pool:
        # Map the fetch_ticker_info function across all securities
        results = pool.map(fetch_ticker_info, securities)

        # Process results and update database
        for ticker, new_sector, new_industry, error in results:
            if error:
                #print(f"Failed to update {ticker}: {error}")
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
                    print(f"Failed to update database for {ticker}: {str(e)}")
                    stats["failed"] += 1
            else:
                stats["unchanged"] += 1

    return stats
