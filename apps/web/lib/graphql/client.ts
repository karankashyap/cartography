import { createClient, fetchExchange, subscriptionExchange } from "@urql/core";
import { createClient as createWSClient } from "graphql-ws";

const GRAPHQL_URL =
  process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/query";
const GRAPHQL_WS_URL =
  process.env.NEXT_PUBLIC_GRAPHQL_WS_URL ?? "ws://localhost:8080/query";

let wsClient: ReturnType<typeof createWSClient> | null = null;

function getWSClient() {
  if (typeof window === "undefined") return null;
  if (!wsClient) {
    wsClient = createWSClient({ url: GRAPHQL_WS_URL });
  }
  return wsClient;
}

export function makeClient() {
  const ws = getWSClient();
  return createClient({
    url: GRAPHQL_URL,
    exchanges: [
      fetchExchange,
      ...(ws
        ? [
            subscriptionExchange({
              forwardSubscription(request) {
                const input = { ...request, query: request.query ?? "" };
                return {
                  subscribe(sink) {
                    const unsubscribe = ws!.subscribe(input, sink);
                    return { unsubscribe };
                  },
                };
              },
            }),
          ]
        : []),
    ],
  });
}
