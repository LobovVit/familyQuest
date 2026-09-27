FROM node:24-alpine AS build

WORKDIR /src

ARG VITE_API_URL=
ENV VITE_API_URL=$VITE_API_URL

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_BUILT_AT=unknown
ENV VITE_APP_VERSION=$APP_VERSION VITE_APP_COMMIT=$APP_COMMIT VITE_APP_BUILT_AT=$APP_BUILT_AT
RUN npm run build

FROM nginx:1.29-alpine

ARG APP_VERSION=dev
ARG APP_COMMIT=unknown
ARG APP_BUILT_AT=unknown
LABEL org.opencontainers.image.version=$APP_VERSION org.opencontainers.image.revision=$APP_COMMIT org.opencontainers.image.created=$APP_BUILT_AT

COPY nginx-main.conf /etc/nginx/nginx.conf
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html

RUN chown -R nginx:nginx /var/cache/nginx /usr/share/nginx/html

USER nginx

EXPOSE 8080
