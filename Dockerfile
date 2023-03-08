FROM golang:1.19-alpine

# Set up apk dependencies
ENV PACKAGES make git libc-dev bash gcc linux-headers eudev-dev curl ca-certificates build-base

ENV GREENFIELD_CHALLENGER_HOME /opt/app

ENV CONFIG_FILE_PATH $GREENFIELD_CHALLENGER_HOME/config/config.json
ENV CONFIG_TYPE "local"
# You need to specify aws s3 config if you want to load config from s3
ENV AWS_REGION ""
ENV AWS_SECRET_KEY ""

# Set working directory for the build
WORKDIR /opt/app

# Add source files
COPY . .

# Install minimum necessary dependencies, remove packages
RUN apk add --no-cache $PACKAGES

# For Private REPO
ARG GH_TOKEN=""
RUN go env -w GOPRIVATE="github.com/bnb-chain/*"
RUN git config --global url."https://${GH_TOKEN}@github.com".insteadOf "https://github.com"

RUN make build

# Run as non-root user for security
USER 1000

VOLUME [ $GREENFIELD_CHALLENGER_HOME ]

# Run the app
CMD ./build/greenfield-challenger --config-type $CONFIG_TYPE --config-path $CONFIG_FILE_PATH --aws-region $A
WS_REGION --aws-secret-key $AWS_SECRET_KEY

