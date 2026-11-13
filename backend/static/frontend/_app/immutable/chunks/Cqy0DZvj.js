import"./DsnmJJEf.js";import{p as f,c as b,f as _,s as y,k as w,d as m,a as h,e as C,h as c,r as l,i as x,g as v,u as g,t as R}from"./WpKdAexE.js";import{s as E,r as S}from"./Bqgkq43j.js";import{I as M}from"./v3EBQyKq.js";import{i as T}from"./BgSVxrKU.js";import{b as H}from"./Bzp-eEHB.js";import{L as N}from"./CrIPYF3_.js";import{g as F}from"./CiHBgu9W.js";import{t as O}from"./BveWepNw.js";import{H as G}from"./DgWmJ36H.js";function z(r,e){f(e,!0);let a=S(e,["$$slots","$$events","$$legacy"]);const s=[["rect",{width:"8",height:"4",x:"8",y:"2",rx:"1",ry:"1"}],["path",{d:"M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"}],["path",{d:"M12 11h4"}],["path",{d:"M12 16h4"}],["path",{d:"M8 11h.01"}],["path",{d:"M8 16h.01"}]];M(r,E({name:"clipboard-list"},()=>a,{get iconNode(){return s},children:(o,n)=>{var t=b(),d=_(t);y(d,()=>e.children??w),m(o,t)},$$slots:{default:!0}})),h()}function I(r,e){f(e,!0);let a=S(e,["$$slots","$$events","$$legacy"]);const s=[["path",{d:"m16 18 6-6-6-6"}],["path",{d:"m8 6-6 6 6 6"}]];M(r,E({name:"code"},()=>a,{get iconNode(){return s},children:(o,n)=>{var t=b(),d=_(t);y(d,()=>e.children??w),m(o,t)},$$slots:{default:!0}})),h()}var L=C('<div class="mb-4 w-full max-w-xl text-left"><p class="mb-2 text-xs text-muted-foreground">Example usage:</p> <div><!></div></div>'),P=C(`<div class="flex flex-col items-center justify-center py-8 text-center"><div class="mb-4 rounded-full bg-muted p-3"><!></div> <h3 class="mb-2 text-lg font-semibold">No Spans Recorded</h3> <p class="mb-4 max-w-md text-sm text-muted-foreground">Spans allow you to track the timing of individual operations within a transaction, such as
		database queries, HTTP calls, or cache operations.</p> <!></div>`);function B(r,e){f(e,!0);function a(i){switch(i){case"gin":return`// In your Gin handler (otelgin puts the request span in c.Request.Context())
func MyHandler(c *gin.Context) {
    ctx, span := otel.Tracer("my-service").Start(c.Request.Context(), "db.query")
    defer span.End()

    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    // ...
}`;case"fiber":return`// In your Fiber handler (otelfiber puts the request span in c.UserContext())
func MyHandler(c *fiber.Ctx) error {
    ctx, span := otel.Tracer("my-service").Start(c.UserContext(), "db.query")
    defer span.End()

    rows, err := db.QueryContext(ctx, "SELECT * FROM users")
    // ...
    return c.JSON(rows)
}`;default:return`ctx, span := otel.Tracer("my-service").Start(ctx, "db.load")
// perform your operations here with ctx
span.End()`}}const s=g(()=>N.includes(e.framework)),o=g(()=>a(e.framework));var n=P(),t=c(n),d=c(t);I(d,{class:"h-6 w-6 text-muted-foreground"}),l(t);var $=x(t,6);{var k=i=>{var u=L(),p=x(c(u),2),q=c(p);G(q,{get language(){return F},get code(){return v(o)}}),l(p),l(u),R(()=>H(p,1,`overflow-hidden rounded-md text-sm ${O.isDark?"dark-code":"light-code"}`)),m(i,u)};T($,i=>{v(s)&&i(k)})}l(n),m(r,n),h()}export{z as C,B as S};
