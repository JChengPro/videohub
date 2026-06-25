FROM node:24-alpine AS build
WORKDIR /src
COPY mobile-frontend/package.json mobile-frontend/package-lock.json ./
RUN npm ci
COPY mobile-frontend/ ./
RUN npm run build

FROM nginx:1.27-alpine
COPY mobile-frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
