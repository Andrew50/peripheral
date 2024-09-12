#FROM python:3.10.1-bullseye
FROM tensorflow/tensorflow:latest
WORKDIR /app
#RUN pip install  tensorflow==2.16.1 --no-cache-dir
RUN apt-get update && apt-get install -y libpq-dev gcc
RUN pip install watchdog
RUN pip install --no-deps tensorflow-serving-api==2.17.0
COPY requirements.txt .
RUN pip install -r requirements.txt --no-cache-dir
COPY . .
ENV PYTHONBUFFERED=1
CMD ["watchmedo", "auto-restart", "--recursive", "--patterns=*py", "--", "python", "-u", "worker.py"]
