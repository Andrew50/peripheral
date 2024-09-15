FROM node:18-alpine as builder
WORKDIR /app
COPY package*.json ./
#RUN npm install --verbose
#RUN npm install -g --unsafe-perm=true --allow-root
#RUN npm install --force
RUN npm install
#COPY . .
#USER node
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
