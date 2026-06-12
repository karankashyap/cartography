"use client";

import { UrqlProvider, ssrExchange } from "@urql/next";
import { useMemo } from "react";
import { makeClient } from "./client";

export function GraphQLProvider({ children }: { children: React.ReactNode }) {
  const [client, ssr] = useMemo(() => {
    const ssr = ssrExchange();
    const client = makeClient();
    return [client, ssr];
  }, []);

  return (
    <UrqlProvider client={client} ssr={ssr}>
      {children}
    </UrqlProvider>
  );
}
