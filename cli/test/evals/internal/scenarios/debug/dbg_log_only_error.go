package debug

import (
	"fmt"
	"time"

	"github.com/tracewayapp/traceway/cli/test/evals/internal/scenarios"
	"github.com/tracewayapp/traceway/cli/test/evals/internal/seed"
)

func init() { scenarios.Register("debug/log-only-error", dbgSeedLogOnlyError) }

// dbgSeedLogOnlyError: v3.4.0 (9h ago) quoted recommendations.timeout_ms in
// config/settings.toml. At startup each instance logs an ERROR ("must be an
// integer, got '800'; recommendations are disabled") and from then on the
// recommendations route answers 500 with an ERROR log and no exception.
// Distractor: the only exception group is a database restart three days ago
// on the product route.
func dbgSeedLogOnlyError(s *seed.Scenario) error {
	deployAt := s.Ago(9 * time.Hour).Truncate(time.Minute).Add(-2 * time.Minute)
	dbDownFrom := s.Ago(3*24*time.Hour + 2*time.Hour).Truncate(time.Minute)
	dbDownTo := dbDownFrom.Add(4 * time.Minute)
	p := s.Project("catalog", seed.OTel)
	api := p.Service("catalog-api", seed.Python("fastapi", "sqlalchemy")).Versions("3.3.2", seed.Deploy{At: deployAt, Version: "3.4.0"})
	hosts := []string{"catalog-api-6c9f7d8b4-2xkqp", "catalog-api-6c9f7d8b4-q7mzt"}
	api.Instances(hosts...)

	// The settings load when uvicorn imports the app; uvicorn's own startup
	// lines go to its non-propagating loggers, not to the OTel handler.
	rolled := deployAt.Add(52 * time.Second)
	for i, h := range hosts {
		api.LogOn(h, deployAt.Add(time.Duration(i)*rolled.Sub(deployAt)), seed.SevError, "invalid config: recommendations.timeout_ms must be an integer, got '800'; recommendations are disabled")
	}
	// Until the second pod restarts, only the first one serves 3.4.0.
	pin := func(r *seed.Req) {
		if t := r.Start(); !t.Before(deployAt) && t.Before(rolled) {
			r.Request().Host = hosts[0]
		}
	}

	dbStack := `Traceback (most recent call last):
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1967, in _exec_single_context
    self.dialect.do_execute(
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/default.py", line 951, in do_execute
    cursor.execute(statement, parameters)
  File "/usr/local/lib/python3.12/site-packages/psycopg/cursor.py", line 97, in execute
    raise ex.with_traceback(None)
psycopg.OperationalError: consuming input failed: server closed the connection unexpectedly
	This probably means the server terminated abnormally
	before or while processing the request.

The above exception was the direct cause of the following exception:

` + dbgStarletteFrames + `  File "/usr/local/lib/python3.12/site-packages/fastapi/routing.py", line 215, in run_endpoint_function
    return await run_in_threadpool(dependant.call, **values)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/starlette/concurrency.py", line 38, in run_in_threadpool
    return await anyio.to_thread.run_sync(func)
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/anyio/to_thread.py", line 65, in run_sync
    return await get_async_backend().run_sync_in_worker_thread(
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/anyio/_backends/_asyncio.py", line 2706, in run_sync_in_worker_thread
    return await future
           ^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/anyio/_backends/_asyncio.py", line 1100, in run
    result = context.run(func, *args)
             ^^^^^^^^^^^^^^^^^^^^^^^^
  File "/app/app/main.py", line 29, in product
    row = conn.execute(text("SELECT id, name, price_cents, currency FROM products WHERE id = :id"), {"id": product_id}).mappings().first()
          ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1419, in execute
    return meth(
           ^^^^^
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/sql/elements.py", line 526, in _execute_on_connection
    return connection._execute_clauseelement(
           ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1641, in _execute_clauseelement
    ret = self._execute_context(
          ^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1846, in _execute_context
    return self._exec_single_context(
           ^^^^^^^^^^^^^^^^^^^^^^^^^^
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1986, in _exec_single_context
    self._handle_dbapi_exception(
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 2355, in _handle_dbapi_exception
    raise sqlalchemy_exception.with_traceback(exc_info[2]) from e
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/base.py", line 1967, in _exec_single_context
    self.dialect.do_execute(
  File "/usr/local/lib/python3.12/site-packages/sqlalchemy/engine/default.py", line 951, in do_execute
    cursor.execute(statement, parameters)
  File "/usr/local/lib/python3.12/site-packages/psycopg/cursor.py", line 97, in execute
    raise ex.with_traceback(None)
sqlalchemy.exc.OperationalError: (psycopg.OperationalError) consuming input failed: server closed the connection unexpectedly
	This probably means the server terminated abnormally
	before or while processing the request.
[SQL: SELECT id, name, price_cents, currency FROM products WHERE id = %(id)s]
[SQL parameters hidden due to hide_parameters=True]
(Background on this error at: https://sqlalche.me/e/20/e3q8)
`
	product := api.Endpoint("GET", "/api/products/{product_id}")
	product.Traffic(seed.Daily(900), seed.Diurnal(), seed.Burst(dbDownFrom, dbDownTo, 8), seed.Over(7*24*time.Hour))
	product.Handler(func(r *seed.Req) {
		pin(r)
		r.Gap(r.Ms(0.5, 1.2))
		at := r.Start()
		if !at.Before(dbDownFrom) && at.Before(dbDownTo) && r.Chance(0.7) {
			r.DB("SELECT id, name, price_cents, currency FROM products WHERE id = %(id)s", r.Ms(1, 4))
			r.Error(seed.Exc{Type: "sqlalchemy.exc.OperationalError", Message: "(psycopg.OperationalError) consuming input failed: server closed the connection unexpectedly\n\tThis probably means the server terminated abnormally\n\tbefore or while processing the request.\n[SQL: SELECT id, name, price_cents, currency FROM products WHERE id = %(id)s]\n[SQL parameters hidden due to hide_parameters=True]\n(Background on this error at: https://sqlalche.me/e/20/e3q8)", Stack: dbStack, Escaped: true})
			r.Status(500)
			return
		}
		r.DB("SELECT id, name, price_cents, currency FROM products WHERE id = %(id)s", r.Ms(1, 5))
		r.Gap(r.Ms(0.3, 0.9))
	})

	recs := api.Endpoint("GET", "/api/products/{product_id}/recommendations")
	recs.Traffic(seed.Daily(800), seed.Diurnal(), seed.Over(7*24*time.Hour))
	recs.Detail(seed.DetailPolicy{Recent: 12 * time.Hour, Rate: 0.02, Max: 15})
	recs.Handler(func(r *seed.Req) {
		pin(r)
		r.Gap(r.Ms(0.4, 1.0))
		if r.Version() == "3.4.0" {
			r.Log(seed.SevError, "recommendations requested but no recommendations client is configured")
			r.Status(500)
			r.Gap(r.Ms(0.2, 0.5))
			return
		}
		path, _ := r.Request().Root.Attrs["url.path"].(string)
		var id int
		fmt.Sscanf(path, "/api/products/%d/recommendations", &id)
		r.HTTP("GET", fmt.Sprintf("http://recommendations.internal:8080/v1/similar?product_id=%d&model=similar-items-v2&limit=12", id), r.LogNormal(45*time.Millisecond, 260*time.Millisecond), 200)
		r.Gap(r.Ms(0.3, 0.9))
	})
	s.Background(api, seed.TypicalTraffic(4))
	for _, h := range hosts {
		api.HostNamed(h).CPU(seed.Flat(0.16, 0.3))
		api.HostNamed(h).Memory(seed.Flat(0.5, 0.57))
	}

	s.Expose("deployAt", deployAt.Format(time.RFC3339))
	s.AfterFlush(func() error {
		hash, err := p.GroupMatching(8*24*time.Hour, "server closed the connection")
		if err != nil {
			return err
		}
		s.Expose("dbHash", hash)
		return nil
	})
	return nil
}
