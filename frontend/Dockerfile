FROM node:20-alpine AS build
WORKDIR /src
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npx shadow-cljs release app

FROM nginx:1.27-alpine
COPY --from=build /src/resources/public /usr/share/nginx/html
EXPOSE 80
