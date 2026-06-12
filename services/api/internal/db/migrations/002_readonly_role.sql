-- +goose Up

CREATE ROLE cartograph_readonly;
GRANT CONNECT ON DATABASE cartograph TO cartograph_readonly;
GRANT USAGE ON SCHEMA public TO cartograph_readonly;
GRANT SELECT ON stores, products, variants, customers, orders, order_items
  TO cartograph_readonly;

CREATE USER cartograph_chat WITH PASSWORD 'cartograph_chat_secret';
GRANT cartograph_readonly TO cartograph_chat;

-- +goose Down
DROP USER IF EXISTS cartograph_chat;
DROP ROLE IF EXISTS cartograph_readonly;
