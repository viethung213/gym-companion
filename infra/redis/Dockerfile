# Official Redis Stack Server configured for FitAI (includes RedisBloom, RedisSearch, etc.)
FROM redis/redis-stack-server:7.2.0-v20

EXPOSE 6379

COPY infra/redis/redis.conf /usr/local/etc/redis/redis.conf

CMD ["redis-stack-server", "/usr/local/etc/redis/redis.conf"]


