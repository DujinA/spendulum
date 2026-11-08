# syntax=docker/dockerfile:1


FROM node:26-alpine AS web
WORKDIR /src
RUN npm install --global pnpm@10.34.9
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY web/package.json web/
COPY e2e/package.json e2e/
RUN pnpm install --frozen-lockfile --filter @spendulum/web
COPY web/ web/
RUN pnpm --filter @spendulum/web build

FROM golang:1.27-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Replace the committed stub with the real SPA build.
RUN rm -rf internal/webui/dist
COPY --from=web /src/web/dist/ internal/webui/dist/
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/spendulum ./cmd/spendulum

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=backend /out/spendulum /usr/local/bin/spendulum
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/spendulum"]
CMD ["serve"]
