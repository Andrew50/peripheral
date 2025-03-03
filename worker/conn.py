import time, psycopg2, redis


class Conn:
    def __init__(self, inside_container=True):
        if inside_container:
            cache_host = "cache"
            db_host = "db"
            tf_host = "http://tf:8501/"
        else:
            cache_host = "localhost"
            db_host = "localhost"
            tf_host = "http://localhost:8501/"
        
        # Connect to database with retries
        self._connect_to_db(db_host)
        
        # Connect to Redis with retries
        self._connect_to_redis(cache_host)
        
        self.tf = tf_host
        self.polygon = "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"

    def _connect_to_db(self, db_host, max_retries=5):
        retry_count = 0
        backoff_time = 1
        
        while retry_count < max_retries:
            try:
                self.db = psycopg2.connect(
                    host=db_host, port="5432", user="postgres", password="pass"
                )
                print("Successfully connected to database", flush=True)
                return
            except psycopg2.OperationalError as e:
                retry_count += 1
                print(f"Database connection attempt {retry_count}/{max_retries} failed: {e}", flush=True)
                if retry_count < max_retries:
                    print(f"Retrying in {backoff_time} seconds...", flush=True)
                    time.sleep(backoff_time)
                    backoff_time *= 2  # Exponential backoff
                else:
                    print("Max retries reached. Could not connect to database.", flush=True)
                    raise
    
    def _connect_to_redis(self, cache_host, max_retries=10):
        retry_count = 0
        backoff_time = 1
        
        while retry_count < max_retries:
            try:
                # Configure Redis with socket timeouts and retry on timeout
                self.cache = redis.Redis(
                    host=cache_host, 
                    port=6379,
                    socket_timeout=5.0,
                    socket_connect_timeout=5.0,
                    socket_keepalive=True,
                    retry_on_timeout=True,
                    health_check_interval=15
                )
                
                # Test the connection with a simple ping
                self.cache.ping()
                print("Successfully connected to Redis", flush=True)
                return
            except (redis.exceptions.ConnectionError, redis.exceptions.ResponseError, redis.exceptions.TimeoutError) as e:
                retry_count += 1
                print(f"Redis connection attempt {retry_count}/{max_retries} failed: {e}", flush=True)
                if retry_count < max_retries:
                    print(f"Retrying in {backoff_time} seconds...", flush=True)
                    time.sleep(backoff_time)
                    backoff_time *= 2  # Exponential backoff
                else:
                    print("Max retries reached. Could not connect to Redis. Will continue to retry in the background.", flush=True)
                    # Instead of raising, create a minimal Redis client that will retry on operations
                    self.cache = redis.Redis(
                        host=cache_host, 
                        port=6379,
                        socket_timeout=10.0,
                        socket_connect_timeout=10.0,
                        retry_on_timeout=True,
                        health_check_interval=30
                    )
                    return

    def check_connection(self):
        try:
            self.cache.ping()
            with self.db.cursor() as cursor:
                cursor.execute("SELECT 1")
        except Exception as e:
            print(f"Connection error: {e}", flush=True)
            print("Attempting to reconnect...", flush=True)
            self.__init__()
