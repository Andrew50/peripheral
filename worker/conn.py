import time, psycopg2, redis, os


class Conn:
    def __init__(self, inside_container=True):
        if inside_container:
            cache_host = os.environ.get("REDIS_HOST", "cache")
            db_host = os.environ.get("DB_HOST", "db")
            tf_host = "http://tf:8501/"
        else:
            cache_host = "localhost"
            db_host = "localhost"
            tf_host = "http://localhost:8501/"
        
        # Get retry configuration from environment variables or use defaults
        self.redis_max_retries = int(os.environ.get("REDIS_RETRY_ATTEMPTS", "5"))
        self.redis_retry_delay = int(os.environ.get("REDIS_RETRY_DELAY", "1"))
        
        # Connect to database with retries
        self._connect_to_db(db_host)
        
        # Connect to Redis with retries
        self._connect_to_redis(cache_host)
        
        self.tf = tf_host
        self.polygon = os.environ.get("POLYGON_API_KEY", "")

    def _connect_to_db(self, db_host, max_retries=5):
        retry_count = 0
        backoff_time = 1
        
        # Get database credentials from environment variables
        db_port = os.environ.get("DB_PORT", "5432")
        db_user = os.environ.get("DB_USER", "postgres")
        db_password = os.environ.get("DB_PASSWORD", "")
        
        while retry_count < max_retries:
            try:
                self.db = psycopg2.connect(
                    host=db_host, 
                    port=db_port, 
                    user=db_user, 
                    password=db_password
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
    
    def _connect_to_redis(self, cache_host, max_retries=None):
        if max_retries is None:
            max_retries = self.redis_max_retries
            
        retry_count = 0
        backoff_time = self.redis_retry_delay
        
        # Get Redis configuration from environment variables
        redis_port = int(os.environ.get("REDIS_PORT", "6379"))
        redis_password = os.environ.get("REDIS_PASSWORD", "")
        
        while retry_count < max_retries:
            try:
                # Create Redis connection with password if available
                if redis_password:
                    self.cache = redis.Redis(
                        host=cache_host, 
                        port=redis_port, 
                        password=redis_password,
                        socket_timeout=5.0,
                        socket_connect_timeout=5.0,
                        retry_on_timeout=True
                    )
                else:
                    self.cache = redis.Redis(
                        host=cache_host, 
                        port=redis_port,
                        socket_timeout=5.0,
                        socket_connect_timeout=5.0,
                        retry_on_timeout=True
                    )
                
                # Test the connection
                self.cache.ping()
                print("Successfully connected to Redis", flush=True)
                return
            except (redis.exceptions.ConnectionError, redis.exceptions.ResponseError) as e:
                retry_count += 1
                print(f"Redis connection attempt {retry_count}/{max_retries} failed: {e}", flush=True)
                if retry_count < max_retries:
                    print(f"Retrying in {backoff_time} seconds...", flush=True)
                    time.sleep(backoff_time)
                    backoff_time *= 2  # Exponential backoff
                else:
                    print("Max retries reached. Could not connect to Redis.", flush=True)
                    raise

    def check_connection(self):
        try:
            self.cache.ping()
            with self.db.cursor() as cursor:
                cursor.execute("SELECT 1")
        except (redis.exceptions.ConnectionError, redis.exceptions.ResponseError) as e:
            print(f"Redis connection error during check: {e}", flush=True)
            print("Attempting to reconnect to Redis...", flush=True)
            try:
                self._connect_to_redis("cache")
            except Exception as e:
                print(f"Failed to reconnect to Redis: {e}", flush=True)
        except psycopg2.OperationalError as e:
            print(f"Database connection error during check: {e}", flush=True)
            print("Attempting to reconnect to database...", flush=True)
            try:
                self._connect_to_db("db")
            except Exception as e:
                print(f"Failed to reconnect to database: {e}", flush=True)
        except Exception as e:
            print(f"Unexpected error during connection check: {e}", flush=True)
            self.__init__()
