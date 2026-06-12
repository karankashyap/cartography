import { createClient, fetchExchange } from "@urql/core";

const GRAPHQL_URL =
  process.env.EXPO_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/query";

export const client = createClient({
  url: GRAPHQL_URL,
  exchanges: [fetchExchange],
});
