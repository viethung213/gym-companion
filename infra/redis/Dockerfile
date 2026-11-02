# Official Redis 7 Alpine configured for FitAI
FROM redis:7-alpine

EXPOSE 6379

COPY infra/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD ["redis-server", "/usr/local/etc/redis/redis.conf"]

