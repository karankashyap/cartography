import type { CodegenConfig } from "@graphql-codegen/cli";

const config: CodegenConfig = {
  schema: "http://localhost:8080/query",
  documents: ["app/**/*.tsx", "components/**/*.tsx", "lib/**/*.ts"],
  generates: {
    "./lib/graphql/generated/": {
      preset: "client",
      plugins: [],
      config: {
        scalars: {
          DateTime: "string",
          Upload: "File",
        },
      },
    },
  },
  ignoreNoDocuments: true,
};

export default config;
