FROM tensorflow/serving
WORKDIR /app
COPY models.config /tmp/models.config
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
#ADD default /models/default/1
ADD default /default-models/default/1
RUN chmod +x /usr/local/bin/entrypoint.sh
#RUN mkdir -p /models && chmod -R 777 /models
RUN mkdir -p /default-models && chmod -R 777 /default-models
#ENV MODEL_BASE_PATH=/models
#ENV MODEL_CONFIG_FILE=/models/models.config
RUN ls -l /models
ENV TF_CPP_MIN_LOG_LEVEL=3
ENV TF_CPP_MIN_VLOG_LEVEL=0
ENV PYTHONFAULTHANDLER=1
EXPOSE 8500
EXPOSE 8501
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

