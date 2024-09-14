FROM postgres:latest
ENV POSTGRES_PASSWORD=pass
COPY init.sql /docker-entrypoint-initdb.d/init.sql
COPY securities.csv /docker-entrypoint-initdb.d/securities.csv
COPY samples.csv /docker-entrypoint-initdb.d/samples.csv
EXPOSE 5432
