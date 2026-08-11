# A Render-specific build of the OPA sidecar for conduit-risk's policies/
# directory. Local dev (docker-compose) bind-mounts policies/ straight into
# the stock openpolicyagent/opa image — Render's build model has no
# equivalent to a bind mount, so this Dockerfile bakes the same policy
# files into the image at build time instead. Same policies, same OPA
# version, same real-server evaluation — only how the files get into the
# container differs.
FROM openpolicyagent/opa:1.19.0
COPY policies /policies
CMD ["run", "--server", "--addr", ":8181", "/policies"]
