# Official lightweight PostgreSQL 16.8 Alpine image configured for FitAI
FROM postgres:16.8-alpine

# Expose standard PostgreSQL port
EXPOSE 5432

# Copy optimized configuration
COPY infra/db/postgres/postgresql.conf /etc/postgresql/postgresql.conf

# Run PostgreSQL using optimized configuration file
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
