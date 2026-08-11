# Render-specific build of the OPA sidecar for conduit-dashboard's own RBAC
# policy — see services/conduit-risk/Dockerfile.opa for why this exists as
# a separate Dockerfile instead of a bind mount (Render's build model has
# no equivalent to docker-compose's volume mount for local dev).
FROM openpolicyagent/opa:1.19.0
COPY policies /policies
CMD ["run", "--server", "--addr", ":8181", "/policies"]
