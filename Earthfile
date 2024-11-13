VERSION 0.8
FROM golang:1.25
WORKDIR /src
ARG --global imagename="ghcr.io/choopm/knxrpc"
ARG --global tag="main"
LABEL org.opencontainers.image.source=https://github.com/choopm/knxrpc

GITHUB_OAUTH:
        FUNCTION
        ARG --required path
        ARG --required secret_name
        RUN --secret TOKEN=$secret_name git config --global \
            url."https://$TOKEN@github.com/$path/".insteadOf \
            "https://github.com/$path/"

GITHUB_SSH:
        FUNCTION
        ARG --required path
        RUN git config --global \
            url."git@github.com:/$path/".insteadOf \
            "https://github.com/$path/"

environment:
        ARG NATIVEOS
        ARG NATIVEARCH

        # install go-task
        RUN curl -sSL https://github.com/go-task/task/releases/latest/download/task_${NATIVEOS}_${NATIVEARCH}.deb \
                -o /tmp/task.deb && \
            dpkg -i /tmp/task.deb && \
            rm /tmp/task.deb

        IF --secret GITHUB_TOKEN=github_token [ "$GITHUB_TOKEN" = "" ]
            RUN apt update && apt -y upgrade && apt install -y openssh-client
            RUN mkdir -p -m 0600 ~/.ssh && \
                ssh-keyscan github.com >> ~/.ssh/known_hosts
            DO +GITHUB_SSH --path=choopm
        ELSE
            DO +GITHUB_OAUTH --path=choopm --secret_name=github_token
        END

        ENV GOPRIVATE=github.com/*

common:
        FROM +environment

        COPY go.* .
        IF --secret GITHUB_TOKEN=github_token [ "$GITHUB_TOKEN" = "" ]
                RUN --ssh go mod download
        ELSE
                RUN go mod download
        END

        COPY . .
        IF --secret GITHUB_TOKEN=github_token [ "$GITHUB_TOKEN" = "" ]
                RUN --ssh task licenses
        ELSE
                RUN task licenses
        END

build:
        FROM +common
        ARG GOOS=
        ARG GOARCH=

        IF --secret GITHUB_TOKEN=github_token [ "$GITHUB_TOKEN" = "" ]
                RUN --ssh task build
        ELSE
                RUN task build
        END

        IF [ "$GOOS" = "windows" ]
                SAVE ARTIFACT cmd/knxrpc/knxrpc /out/usr/bin/knxrpc AS LOCAL build/knxrpc-$GOOS-$GOARCH.exe
        ELSE
                SAVE ARTIFACT cmd/knxrpc/knxrpc /out/usr/bin/knxrpc AS LOCAL build/knxrpc-$GOOS-$GOARCH
        END
        SAVE ARTIFACT cmd/knxrpc/knxrpc.yaml /out/etc/knxrpc.yaml AS LOCAL build/knxrpc.yaml
        SAVE ARTIFACT LICENSE /out/usr/share/licenses/knxrpc/LICENSE AS LOCAL build/LICENSE
        SAVE ARTIFACT NOTICE /out/usr/share/licenses/knxrpc/NOTICE AS LOCAL build/NOTICE
        SAVE ARTIFACT LICENSES /out/usr/share/licenses/knxrpc/LICENSES AS LOCAL build/LICENSES

base-release:
        FROM gcr.io/distroless/static:nonroot # debug-nonroot includes busybox
        ENTRYPOINT ["/usr/bin/knxrpc"]
        CMD ["-f", "/etc/knxrpc.yaml", "server"]

release-amd64:
        BUILD +build --GOARCH=amd64 --GOOS=linux
        FROM --platform=linux/amd64 +base-release
        COPY (+build/out/ --GOARCH=amd64) /
        SAVE IMAGE --push $imagename:$tag

release-arm64:
        BUILD +build --GOARCH=arm64 --GOOS=linux
        FROM --platform=linux/arm64 +base-release
        COPY (+build/out/ --GOARCH=arm64) /
        SAVE IMAGE --push $imagename:$tag

release-arm:
        BUILD +build --GOARCH=arm --GOOS=linux
        FROM --platform=linux/arm +base-release
        COPY (+build/out/ --GOARCH=arm) /
        SAVE IMAGE --push $imagename:$tag

# release-local creates an image for the current platform
release-local:
        BUILD +build
        FROM +base-release
        COPY +build/out/ /
        SAVE IMAGE --push $imagename:local

# release creates a multi platform image and binaries
release:
        BUILD +release-amd64
        BUILD +release-arm64
        BUILD +release-arm
        BUILD +build --GOARCH=amd64 --GOOS=windows
        BUILD +build --GOARCH=amd64 --GOOS=darwin
        BUILD +build --GOARCH=arm64 --GOOS=windows
        BUILD +build --GOARCH=arm64 --GOOS=darwin
