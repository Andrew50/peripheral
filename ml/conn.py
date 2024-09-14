import time, psycopg2, redis

class Conn:

    def __init__(self, inside_container = True):
        if inside_container:
            cache_host = 'cache'
            db_host = 'db'
            tf_host = 'http://tf:8501/'
        else:
            cache_host = 'localhost'
            db_host = 'localhost'
            tf_host = 'http://localhost:8501/'
        while True:
            try:
                self.cache = redis.Redis(host=cache_host, port=6379)
                self.db = psycopg2.connect(host=db_host,port='5432',user='postgres',password='pass')
                self.tf = tf_host
                self.polygon = "ogaqqkwU1pCi_x5fl97pGAyWtdhVLJYm"
            except  psycopg2.OperationalError:
                print("waiting for db", flush = True)
                time.sleep(5)
            else:
                break


    def check_connection(self):
        try:
            self.cache.ping()
            self.db.ping()
        except:
            print('Connection error')
            self.__init__()
