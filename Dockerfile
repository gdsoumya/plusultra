FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /app
COPY app/package.json app/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY app .
RUN CI=1 npx expo export --platform web

# The catalog datasets are fetched here, so building needs network access. Each file is pinned by
# checksum; a new dataset release changes /seed/VERSION, which makes the server reseed on start.
FROM --platform=$BUILDPLATFORM golang:1.27 AS seed
ADD --checksum=sha256:ad2770ed89335c8b79da12854cc684d419e0f482de69b826c4a31bfac2405eab \
    https://codeload.github.com/yuhonas/free-exercise-db/zip/f00c92c7dcf1216a928a52c3706c7ce8e2f71ed5 /dl/free-exercise-db.zip
ADD --checksum=sha256:b80817294b8850530aaedf2e515c02593b1824f763a0ff356e5c2081643e6fd0 \
    https://fdc.nal.usda.gov/fdc-datasets/FoodData_Central_sr_legacy_food_csv_2018-04.zip /seed/sr.zip
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/seedprep cmd/seedprep
RUN --mount=type=cache,target=/root/.cache/go-build go build -o /seedprep ./cmd/seedprep \
    && /seedprep /dl/free-exercise-db.zip /seed/exercises \
    && sha256sum /dl/free-exercise-db.zip /seed/sr.zip | cut -d' ' -f1 | sha256sum | cut -c1-12 > /seed/VERSION \
    && chmod -R a+rX /seed

FROM --platform=$BUILDPLATFORM golang:1.27 AS build
ARG VERSION=dev
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=web /app/dist internal/web/dist
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X github.com/gdsoumya/plusultra/internal/version.Version=$VERSION" \
    -o /plusultra ./cmd/plusultra
RUN mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /plusultra /plusultra
COPY --from=seed /seed /seed
# Named volumes inherit the owner of the image directory, so nonroot can write images and photos.
COPY --from=build --chown=nonroot:nonroot /data /data
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/plusultra"]
CMD ["serve"]
