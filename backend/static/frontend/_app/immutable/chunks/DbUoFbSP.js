import"./DsnmJJEf.js";import{p as f,j as _,f as b,y,e as m,a as h,o as c,r as l,k as x,g as v,u as g,l as R,h as w}from"./CKfuj5eA.js";import{s as C}from"./D5sFTGVp.js";import{s as S,r as E}from"./MFDFtFUE.js";import{I as M}from"./DpqLpM4u.js";import{i as T}from"./BkUOt51i.js";import{a as H}from"./DI9NF4bQ.js";import{L as N}from"./DgFyTHSd.js";import{g as F}from"./CiHBgu9W.js";import{t as O}from"./BVuJs0XO.js";import{H as G}from"./CdF7FAmL.js";var I=new Set(["$$slots","$$events","$$legacy"]);function Z(r,e){f(e,!0);let a=E(e,I);const s=[["rect",{width:"8",height:"4",x:"8",y:"2",rx:"1",ry:"1"}],["path",{d:"M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"}],["path",{d:"M12 11h4"}],["path",{d:"M12 16h4"}],["path",{d:"M8 11h.01"}],["path",{d:"M8 16h.01"}]];M(r,S({name:"clipboard-list"},()=>a,{get iconNode(){return s},children:(o,n)=>{var t=_(),d=b(t);C(d,()=>e.children??y),m(o,t)},$$slots:{default:!0}})),h()}var L=new Set(["$$slots","$$events","$$legacy"]);function P(r,e){f(e,!0);let a=E(e,L);const s=[["path",{d:"m16 18 6-6-6-6"}],["path",{d:"m8 6-6 6 6 6"}]];M(r,S({name:"code"},()=>a,{get iconNode(){return s},children:(o,n)=>{var t=_(),d=b(t);C(d,()=>e.children??y),m(o,t)},$$slots:{default:!0}})),h()}var j=w('<div class="mb-4 w-full max-w-xl text-left"><p class="mb-2 text-xs text-muted-foreground">Example usage:</p> <div><!></div></div>'),A=w(`<div class="flex flex-col items-center justify-center py-8 text-center"><div class="mb-4 rounded-full bg-muted p-3"><!></div> <h3 class="mb-2 text-lg font-semibold">No Spans Recorded</h3> <p class="mb-4 max-w-md text-sm text-muted-foreground">Spans allow you to track the timing of individual operations within a transaction, such as
		database queries, HTTP calls, or cache operations.</p> <!></div>`);function ee(r,e){f(e,!0);function a(i){switch(i){case"gin":return`// In your Gin handler (otelgin puts the request span in c.Request.Context())
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
span.End()`}}const s=g(()=>N.includes(e.framework)),o=g(()=>a(e.framework));var n=A(),t=c(n),d=c(t);P(d,{class:"h-6 w-6 text-muted-foreground"}),l(t);var $=x(t,6);{var k=i=>{var u=j(),p=x(c(u),2),q=c(p);G(q,{get language(){return F},get code(){return v(o)}}),l(p),l(u),R(()=>H(p,1,`overflow-hidden rounded-md text-sm ${O.isDark?"dark-code":"light-code"}`)),m(i,u)};T($,i=>{v(s)&&i(k)})}l(n),m(r,n),h()}export{Z as C,ee as S};
