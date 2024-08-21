FROM node:18-alpine as builder
WORKDIR /app
COPY package.json .
RUN npm install
COPY . .
EXPOSE 5000
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0"]
