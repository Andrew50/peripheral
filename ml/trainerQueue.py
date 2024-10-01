import random
from psycopg2.extras import execute_values
from datetime import datetime, timedelta
from screen import screen

def generateSamples(conn, setupId):
    start_date = datetime(2004, 1, 1)
    end_date = datetime.now()
    random_date = start_date + timedelta(days=random.randint(0, (end_date - start_date).days))
    print(f"fetching from {random_date}")
    results = screen(conn, [setupId], random_date, threshold=0)
    closest_to_50 = sorted(results, key=lambda x: abs(x['score'] - 50))
    selected_instances = closest_to_50[:20]
    samples = [(setupId, instance['securityId'], datetime.fromtimestamp(instance['timestamp'] / 1000)) for instance in selected_instances]
    return samples

def refillTrainerQueue(conn, setupId):
    err = None
    samplesGot = 0
    try:
        samples = generateSamples(conn, setupId)
        if not samples:
            print("-------------------------------- no samples found from queue refill")
            raise Exception()
        insert_query = """
            INSERT INTO samples (setupId, securityId, timestamp, label)
            VALUES %s
            ON CONFLICT DO NOTHING
        """
        formatted_values = [(sample[0], sample[1], sample[2], None) for sample in samples]  # Add None for label
        with conn.db.cursor() as cursor:
            execute_values(cursor, insert_query, formatted_values)
        samplesGot = len(formatted_values)
        conn.db.commit()
    except Exception as e:
        err = e
    finally:
        conn.cache.set(f"{setupId}_queue_running", "false")
        if err is not None:
            raise err

    return samplesGot

