# Stage 1: Build static assets
FROM node:20-alpine AS builder
WORKDIR /app
ARG VITE_READ_API_KEY
ENV VITE_READ_API_KEY=$VITE_READ_API_KEY
COPY package*.json ./
RUN npm install
COPY . .
RUN npm run build

# Stage 2: Serve assets with Nginx
FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]