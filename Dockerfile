FROM public.ecr.aws/docker/library/golang:1.18.7
ARG MAXMIND_LICENSE_KEY
ARG GITHUB_TOKEN
RUN apt-get update && apt-get install -y jq libhwloc-dev ocl-icd-opencl-dev make wget pkg-config hwloc git traceroute jc
WORKDIR /app
RUN curl https://sh.rustup.rs -sSf | bash -s -- -y
ENV PATH="/root/.cargo/bin:${PATH}"
RUN git clone https://github.com/data-preservation-programs/ValidationBot.git .
RUN git checkout ${CODEBUILD_RESOLVED_SOURCE_VERSION}

# turning maxmind off so it doesnt clog tests when
# license has been hit too many times:
# https://github.com/data-preservation-programs/ValidationBot/actions/runs/3943694005/jobs/6749126788#step:14:736
# RUN make maxmind
RUN make deps
RUN make build

EXPOSE 80
EXPOSE 7999
EXPOSE 7998
CMD ["/app/validation_bot", "run"]
