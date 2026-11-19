"""Tracer for spans around in-process work; the distro exports them."""

from opentelemetry import trace

tracer = trace.get_tracer("members-api")
