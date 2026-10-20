package com.example.shop.web;

import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.filter.OncePerRequestFilter;
import org.springframework.web.servlet.HandlerMapping;

/** One line per request ("POST /api/checkout 200 412ms"), inside the request's trace. */
@Component
public class AccessLogFilter extends OncePerRequestFilter {

    private static final Logger log = LoggerFactory.getLogger("access");

    @Override
    protected void doFilterInternal(HttpServletRequest req, HttpServletResponse res, FilterChain chain)
            throws ServletException, IOException {
        long start = System.nanoTime();
        boolean failed = false;
        try {
            chain.doFilter(req, res);
        } catch (IOException | ServletException | RuntimeException e) {
            failed = true;
            throw e;
        } finally {
            long ms = (System.nanoTime() - start) / 1_000_000;
            // An exception that escapes the chain becomes a 500 only after this filter returns.
            int status = failed ? HttpServletResponse.SC_INTERNAL_SERVER_ERROR : res.getStatus();
            Object route = req.getAttribute(HandlerMapping.BEST_MATCHING_PATTERN_ATTRIBUTE);
            log.atInfo()
                    .addKeyValue("http.request.method", req.getMethod())
                    .addKeyValue("url.path", req.getRequestURI())
                    .addKeyValue("http.route", route == null ? "" : route.toString())
                    .addKeyValue("http.response.status_code", status)
                    .addKeyValue("duration_ms", ms)
                    .log("{} {} {} {}ms", req.getMethod(), req.getRequestURI(), status, ms);
        }
    }
}
