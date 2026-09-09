FROM node:20-alpine AS build
# shadow-cljs is a JVM tool (compiles ClojureScript) — node:20-alpine has no
# JDK/JRE by default, which fails "shadow-cljs release" with "Executable
# 'java' not found on system path" until this is added.
RUN apk add --no-cache openjdk11-jre-headless
WORKDIR /src
COPY package.json package-lock.json* ./
RUN npm install
COPY . .
RUN npx shadow-cljs release app

FROM nginx:1.27-alpine
COPY --from=build /src/resources/public /usr/share/nginx/html
EXPOSE 80
