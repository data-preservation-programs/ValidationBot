FROM public.ecr.aws/docker/library/golang:1.18.7
RUN apt-get update && apt-get install -y jq libhwloc-dev ocl-icd-opencl-dev make wget pkg-config hwloc
WORKDIR /app
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN cargo --help
COPY . ./
RUN make deps
RUN make build

EXPOSE 80
EXPOSE 7999
EXPOSE 7998
CMD ["/app/validation_bot", "run"]
