import pandas as pd
from datetime import datetime
import os


def refresh_statistics(trade_df):
    # Create a list to store P/L values
    pnl_values = []

    for i in range(len(trade_df)):
        trade = trade_df.iloc[i]

        total_entry_value = 0
        total_entry_shares = 0
        for j in range(1, 21):
            entry_price = trade[f"Entry Price {j}"]
            entry_shares = trade[f"Entry Shares {j}"]
            if pd.notna(entry_price) and pd.notna(entry_shares):
                total_entry_value += float(entry_price) * float(entry_shares)
                total_entry_shares += float(entry_shares)

        total_exit_value = 0
        total_exit_shares = 0
        for j in range(1, 51):
            exit_price = trade[f"Exit Price {j}"]
            exit_shares = trade[f"Exit Shares {j}"]
            if pd.notna(exit_price) and pd.notna(exit_shares):
                total_exit_value += float(exit_price) * float(exit_shares)
                total_exit_shares += float(exit_shares)

        # Calculate P/L based on trade direction
        pnl = None
        if trade["Trade Direction"] == "Long":
            if total_exit_shares > 0:  # Only calculate if we have exits
                pnl = round(
                    total_exit_value
                    - (total_entry_value * (total_exit_shares / total_entry_shares)),
                    2,
                )
        else:  # Short
            if total_exit_shares > 0:  # Only calculate if we have exits
                pnl = (
                    total_entry_value * (total_exit_shares / total_entry_shares)
                ) - total_exit_value

        ticker = trade["Ticker"]
        if pnl is not None and len(ticker) > 6 and any(c in ticker for c in ["C", "P"]):
            pnl = pnl * 100  # Multiply P/L by 100 for options

        pnl_values.append(round(pnl, 2) if pnl is not None else None)

    # Update the DataFrame all at once
    trade_df["Closed P/L"] = pnl_values


def process_data(cv):
    df = pd.read_csv(cv, skiprows=3, dtype={"Order Time": str})

    # Initialize trade_df with all necessary columns
    columns = [
        "Ticker",
        "Trade Direction",
        "Date",
        "Status",
        "Open Quantity",
        "Closed P/L",
    ]

    # Add Entry columns
    for i in range(1, 21):
        columns.extend([f"Entry Time {i}", f"Entry Price {i}", f"Entry Shares {i}"])

    # Add Exit columns
    for i in range(1, 51):
        columns.extend([f"Exit Time {i}", f"Exit Price {i}", f"Exit Shares {i}"])

    if os.path.exists("trade_df.csv"):
        trade_df = pd.read_csv("trade_df.csv")
    else:
        trade_df = pd.DataFrame(columns=columns)

    # Drop any rows where all columns are NaN
    df = df.dropna(how="all")

    for i in range(len(df) - 1, -1, -1):
        trade = df.iloc[i]
        open_trade_row = check_if_open_trade(trade_df, trade["Symbol"])
        if open_trade_row is not None:
            update_trade(trade_df, trade, open_trade_row)
        else:
            create_trade(trade_df, trade)
    refresh_statistics(trade_df)
    trade_df.to_csv("trade_df.csv", index=False)


def update_trade(trade_df, trade, open_trade_row):
    trade_direction = open_trade_row["Trade Direction"]
    new_trade_direction = ""
    if trade_direction == "Short":
        if (
            "Short" in trade["Trade Description"]
            or "Sell to Open" in trade["Trade Description"]
        ):
            new_trade_direction = "Entry"
        else:
            new_trade_direction = "Exit"
    elif trade_direction == "Long":
        if (
            "Buy" in trade["Trade Description"]
            or "Buy to Open" in trade["Trade Description"]
        ):
            new_trade_direction = "Entry"
        else:
            new_trade_direction = "Exit"

    # Find the latest non-null entry/exit number
    if new_trade_direction == "Entry":
        # Look for the first empty entry slot
        for i in range(1, 21):
            if pd.isna(open_trade_row[f"Entry Time {i}"]):
                # Found an empty slot, update it
                trade_dt, _ = parse_datetime(trade["Order Time"])
                trade_df.at[open_trade_row.name, f"Entry Time {i}"] = trade_dt
                shares = 0
                if "Limit" in trade["Trade Description"]:
                    if "FILLED AT" in trade["Status"]:
                        trade_df.at[open_trade_row.name, f"Entry Price {i}"] = trade[
                            "Status"
                        ].split("$")[1]
                        shares = float(trade["Quantity"].replace(",", ""))
                    elif "PARTIAL" in trade["Status"]:
                        status_parts = trade["Status"].split("\n")
                        if (
                            len(status_parts) >= 4
                        ):  # Assuming partial fill info is on the 4th line
                            shares = float(
                                status_parts[3].split(" ")[0].replace(",", "")
                            )
                            trade_df.at[open_trade_row.name, f"Entry Price {i}"] = (
                                trade["Trade Description"].split("$")[1]
                            )
                elif "Market" in trade["Trade Description"]:
                    trade_df.at[open_trade_row.name, f"Entry Price {i}"] = trade[
                        "Status"
                    ].split("$")[1]
                    shares = float(trade["Quantity"].replace(",", ""))
                trade_df.at[open_trade_row.name, f"Entry Shares {i}"] = shares
                if trade_direction == "Short":
                    trade_df.at[open_trade_row.name, f"Open Quantity"] = (
                        float(trade_df.at[open_trade_row.name, f"Open Quantity"])
                        - shares
                    )
                elif trade_direction == "Long":
                    trade_df.at[open_trade_row.name, f"Open Quantity"] = (
                        float(trade_df.at[open_trade_row.name, f"Open Quantity"])
                        + shares
                    )
                break
    elif new_trade_direction == "Exit":
        # Look for the first empty exit slot
        for i in range(1, 51):
            if pd.isna(open_trade_row[f"Exit Time {i}"]):
                # Found an empty slot, update it
                trade_dt, _ = parse_datetime(trade["Order Time"])
                trade_df.at[open_trade_row.name, f"Exit Time {i}"] = trade_dt
                shares = 0
                if (
                    "Limit" in trade["Trade Description"]
                    or "Stop" in trade["Trade Description"]
                ):
                    if "FILLED AT" in trade["Status"]:
                        trade_df.at[open_trade_row.name, f"Exit Price {i}"] = trade[
                            "Status"
                        ].split("$")[1]
                        shares = float(trade["Quantity"].replace(",", ""))
                    elif "PARTIAL" in trade["Status"]:
                        status_parts = trade["Status"].split("\n")
                        if (
                            len(status_parts) >= 4
                        ):  # Assuming partial fill info is on the 4th line
                            shares = float(
                                status_parts[3].split(" ")[0].replace(",", "")
                            )
                            trade_df.at[open_trade_row.name, f"Exit Price {i}"] = trade[
                                "Trade Description"
                            ].split("$")[1]
                elif "Market" in trade["Trade Description"]:
                    trade_df.at[open_trade_row.name, f"Exit Price {i}"] = trade[
                        "Status"
                    ].split("$")[1]
                    shares = float(trade["Quantity"].replace(",", ""))
                trade_df.at[open_trade_row.name, f"Exit Shares {i}"] = shares
                if trade_direction == "Short":
                    trade_df.at[open_trade_row.name, f"Open Quantity"] = (
                        float(trade_df.at[open_trade_row.name, f"Open Quantity"])
                        + shares
                    )
                elif trade_direction == "Long":
                    trade_df.at[open_trade_row.name, f"Open Quantity"] = (
                        float(trade_df.at[open_trade_row.name, f"Open Quantity"])
                        - shares
                    )
                break
    if trade_df.at[open_trade_row.name, f"Open Quantity"] == 0:
        trade_df.at[open_trade_row.name, f"Status"] = "Closed"


def create_trade(trade_df, trade):

    trade_direction = ""
    fidelity_trade_status = trade["Status"]
    trade_description = trade["Trade Description"]
    if "Short" in trade_description or "Sell to Open" in trade_description:
        trade_direction = "Short"
    else:
        trade_direction = "Long"
    trade_dt, trade_date = parse_datetime(trade["Order Time"])
    new_row = {
        "Ticker": trade["Symbol"],
        "Trade Direction": trade_direction,
        "Date": trade_date,
        "Status": "Open",
        "Open Quantity": None,
    }
    for i in range(1, 21):
        new_row[f"Entry Time {i}"] = None
        new_row[f"Entry Price {i}"] = None
        new_row[f"Entry Shares {i}"] = None

    for i in range(1, 51):
        new_row[f"Exit Time {i}"] = None
        new_row[f"Exit Price {i}"] = None
        new_row[f"Exit Shares {i}"] = None
    new_row["Entry Time 1"] = trade_dt
    if "Limit" in trade_description:
        if "FILLED AT" in fidelity_trade_status:
            new_row["Entry Price 1"] = fidelity_trade_status.split("$")[1]
            new_row["Entry Shares 1"] = float(trade["Quantity"].replace(",", ""))
            new_row["Open Quantity"] = float(trade["Quantity"].replace(",", ""))
        elif "PARTIAL" in fidelity_trade_status:
            shares = float(
                fidelity_trade_status.split("\n")[3].split(" ")[0].replace(",", "")
            )
            new_row["Entry Shares 1"] = shares
            description_split = trade_description.split("$")
            if len(description_split) == 2:
                new_row["Entry Price 1"] = description_split[1]
            elif len(description_split) == 3:
                new_row["Entry Price 1"] = description_split[2]
            new_row["Open Quantity"] = shares
    elif "Market" in trade_description:
        new_row["Entry Price 1"] = fidelity_trade_status.split("$")[1]
        new_row["Entry Shares 1"] = float(trade["Quantity"].replace(",", ""))
        new_row["Open Quantity"] = float(trade["Quantity"].replace(",", ""))
    if trade_direction == "Short":
        new_row["Open Quantity"] = -float(new_row["Open Quantity"])
    trade_df.loc[len(trade_df)] = new_row


def check_if_open_trade(df, ticker):
    if df.empty:
        return None
    ticker_trades = (df["Ticker"] == ticker) & (df["Status"] == "Open")
    open_trades = df[ticker_trades]

    return open_trades.iloc[-1] if not open_trades.empty else None


def parse_datetime(datetime_str):
    datetime_str = " ".join(datetime_str.split())
    try:
        dt = datetime.strptime(datetime_str, "%I:%M:%S %p %m/%d/%Y")
        date_only_str = dt.date().strftime("%m/%d/%Y")

        return dt, date_only_str
    except ValueError as e:
        print(f"Error parsing datetime: {e}")
        return None, None


if __name__ == "__main__":
    trade_df = pd.read_csv("trade_df.csv")
    process_data(cv="2025-01-28.csv")
    refresh_statistics(trade_df)
