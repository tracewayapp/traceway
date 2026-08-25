import{c as ke,p as fe,t as Ot,f as Ve}from"./CQVUojPQ.js";import"./DsnmJJEf.js";import{p as Oe,k as x,f as S,m as u,r as b,v as E,g as t,u as U,o as ie,q as oe,b as s,a as xe,c as te,l as ee,t as be,j as me,e as Te,ap as xt,h as Ze}from"./CiUREmk6.js";import{C as pe,b as _e,a as Ct,c as It,e as Mt,f as $t,h as Lt,O as we,i as Dt,s as Pt,j as kt}from"./CUWOcfuG.js";import{i as ae}from"./Bi5UrFVz.js";import{e as Re}from"./CJYOg0qA.js";import{c as ne}from"./B4WV8lP6.js";import{d as Bt}from"./DTxW3KaW.js";import{a as Ut}from"./BUHkGzeD.js";import{a as Le,b as De,T as Pe}from"./yCmOliy9.js";import{g as Ft}from"./CiHBgu9W.js";import{C as Gt}from"./rZ2L8Jhm.js";import{C as Kt}from"./CSt62gYR.js";import{C as Ht}from"./CtiUukZM.js";import{C as zt,a as Wt}from"./BN1q10Fr.js";import{B as Ae}from"./p_aYziH4.js";import{L as qt}from"./XHvEfLdG.js";import{A as Zt,a as Yt,b as Xt,c as Vt,d as Qt,e as Jt}from"./BFscXS-z.js";import{C as jt}from"./6O0_8XCu.js";import{R as ea}from"./CgFm_jGm.js";import{K as Qe}from"./BLPBjNMd.js";import{C as Ye}from"./AU_yl07A.js";const Xe="[A-Za-z$_][0-9A-Za-z$_]*",ta=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],aa=["true","false","null","undefined","NaN","Infinity"],Je=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],je=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],et=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],ra=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],na=[].concat(et,Je,je);function oa(e){const a=e.regex,n=(c,{after:T})=>{const $="</"+c[0].slice(1);return c.input.indexOf($,T)!==-1},r=Xe,l={begin:"<>",end:"</>"},w=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,T)=>{const $=c[0].length+c.index,z=c.input[$];if(z==="<"||z===","){T.ignoreMatch();return}z===">"&&(n(c,{after:$})||T.ignoreMatch());let W;const q=c.input.substring($);if(W=q.match(/^\s*=/)){T.ignoreMatch();return}if((W=q.match(/^\s+extends\s+/))&&W.index===0){T.ignoreMatch();return}}},i={$pattern:Xe,keyword:ta,literal:aa,built_in:na,"variable.language":ra},g="[0-9](_?[0-9])*",p=`\\.(${g})`,y="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",O={className:"number",variants:[{begin:`(\\b(${y})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${y})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:i,contains:[]},f={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},A={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},L={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},D={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:r+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,f,A,_,L,{match:/\$\d+/},O];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:i,contains:["self"].concat(m)});const v=[].concat(D,d.contains),N=v.concat([{begin:/(\s*)\(/,end:/\)/,keywords:i,contains:["self"].concat(v)}]),R={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:N},G={variants:[{match:[/class/,/\s+/,r,/\s+/,/extends/,/\s+/,a.concat(r,"(",a.concat(/\./,r),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,r],scope:{1:"keyword",3:"title.class"}}]},I={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...Je,...je]}},X={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,r,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[R],illegal:/%/},re={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(c){return a.concat("(?!",c.join("|"),")")}const K={match:a.concat(/\b/,ce([...et,"super","import"].map(c=>`${c}\\s*\\(`)),r,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},M={begin:a.concat(/\./,a.lookahead(a.concat(r,/(?![0-9A-Za-z$_(])/))),end:r,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},h={match:[/get|set/,/\s+/,r,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},R]},P="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",k={match:[/const|var|let/,/\s+/,r,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(P)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[R]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:i,exports:{PARAMS_CONTAINS:N,CLASS_REFERENCE:I},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),X,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,f,A,_,L,D,{match:/\$\d+/},O,I,{scope:"attr",match:r+a.lookahead(":"),relevance:0},k,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[D,e.REGEXP_MODE,{className:"function",begin:P,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:N}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:w},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[R,e.inherit(e.TITLE_MODE,{begin:r,className:"title.function"})]},{match:/\.\.\./,relevance:0},M,{match:"\\$"+r,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[R]},K,re,G,h,{match:/\$[(.]/}]}}const Ne={name:"javascript",register:oa};function sa(e){const a=e.regex,n=/(?![A-Za-z0-9])(?![$])/,r=a.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,n),l=a.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,n),w=a.concat(/[A-Z]+/,n),o={scope:"variable",match:"\\$+"+r},i={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},g={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},p=e.inherit(e.APOS_STRING_MODE,{illegal:null}),y=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(g)}),O={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(g),"on:begin":(M,h)=>{h.data._beginMatch=M[1]||M[2]},"on:end":(M,h)=>{h.data._beginMatch!==M[1]&&h.ignoreMatch()}},d=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),f=`[ 	
]`,A={scope:"string",variants:[y,p,O,d]},_={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},L=["false","null","true"],B=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],D=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],v={keyword:B,literal:(M=>{const h=[];return M.forEach(P=>{h.push(P),P.toLowerCase()===P?h.push(P.toUpperCase()):h.push(P.toLowerCase())}),h})(L),built_in:D},N=M=>M.map(h=>h.replace(/\|\d+$/,"")),R={variants:[{match:[/new/,a.concat(f,"+"),a.concat("(?!",N(D).join("\\b|"),"\\b)"),l],scope:{1:"keyword",4:"title.class"}}]},G=a.concat(r,"\\b(?!\\()"),I={variants:[{match:[a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[l,a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{1:"title.class",3:"variable.constant"}},{match:[l,a.concat("::",a.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[l,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},X={scope:"attr",match:a.concat(r,a.lookahead(":"),a.lookahead(/(?!::)/))},J={relevance:0,begin:/\(/,end:/\)/,keywords:v,contains:[X,o,I,e.C_BLOCK_COMMENT_MODE,A,_,R]},re={relevance:0,match:[/\b/,a.concat("(?!fn\\b|function\\b|",N(B).join("\\b|"),"|",N(D).join("\\b|"),"\\b)"),r,a.concat(f,"*"),a.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[J]};J.contains.push(re);const ce=[X,I,e.C_BLOCK_COMMENT_MODE,A,_,R],K={begin:a.concat(/#\[\s*\\?/,a.either(l,w)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:L,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:L,keyword:["new","array"]},contains:["self",...ce]},...ce,{scope:"meta",variants:[{match:l},{match:w}]}]};return{case_insensitive:!1,keywords:v,contains:[K,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},i,{scope:"variable.language",match:/\$this\b/},o,re,I,{match:[/const/,/\s/,r],scope:{1:"keyword",3:"variable.constant"}},R,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:v,contains:["self",K,o,I,e.C_BLOCK_COMMENT_MODE,A,_]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},A,_]}}const Sr={name:"php",register:sa};function ia(e){const a=e.regex,n=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),r=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],i={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:r,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},g={className:"meta",begin:/^(>>>|\.\.\.) /},p={className:"subst",begin:/\{/,end:/\}/,keywords:i,illegal:/#/},y={begin:/\{\{/,relevance:0},O={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g,y,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g,y,p]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,y,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,y,p]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},d="[0-9](_?[0-9])*",f=`(\\b(${d}))?\\.(${d})|\\b(${d})\\.`,A=`\\b|${r.join("|")}`,_={className:"number",relevance:0,variants:[{begin:`(\\b(${d})|(${f}))[eE][+-]?(${d})[jJ]?(?=${A})`},{begin:`(${f})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${A})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${A})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${A})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${A})`},{begin:`\\b(${d})[jJ](?=${A})`}]},L={className:"comment",begin:a.lookahead(/# type:/),end:/$/,keywords:i,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},B={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:["self",g,_,O,e.HASH_COMMENT_MODE]}]};return p.contains=[O,_,g],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:i,illegal:/(<\/|\?)|=>/,contains:[g,_,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},O,L,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[B]},{variants:[{match:[/\bclass/,/\s+/,n,/\s*/,/\(\s*/,n,/\s*\)/]},{match:[/\bclass/,/\s+/,n]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[_,B,O]}]}}const ca={name:"python",register:ia};function wr(e){const a="go get go.tracewayapp.com";switch(e){case"gin":return`${a} && go get go.tracewayapp.com/tracewaygin`;case"chi":return`${a} && go get go.tracewayapp.com/tracewaychi`;case"fiber":return`${a} && go get go.tracewayapp.com/tracewayfiber`;case"fasthttp":return`${a} && go get go.tracewayapp.com/tracewayfasthttp`;case"stdlib":return`${a} && go get go.tracewayapp.com/tracewayhttp`;case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.1")';case"ios":return'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';default:return a}}function Ar(e,a,n){const r=a?`${a}@${n}/api/report`:`YOUR_TOKEN@${n}/api/report`;switch(e){case"gin":return`package main

import (
    "github.com/gin-gonic/gin"
    tracewaygin "go.tracewayapp.com/tracewaygin"
)

func main() {
    r := gin.Default()
    r.Use(tracewaygin.New("${r}"))
    r.Run(":8080")
}`;case"chi":return`package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    tracewaychi "go.tracewayapp.com/tracewaychi"
)

func main() {
    r := chi.NewRouter()
    r.Use(tracewaychi.New("${r}"))

    r.Get("/api/users", getUsers)
    http.ListenAndServe(":8080", r)
}`;case"fiber":return`package main

import (
    "github.com/gofiber/fiber/v2"
    tracewayfiber "go.tracewayapp.com/tracewayfiber"
)

func main() {
    app := fiber.New()
    app.Use(tracewayfiber.New("${r}"))

    app.Get("/api/users", getUsers)
    app.Listen(":8080")
}`;case"fasthttp":return`package main

import (
    "github.com/valyala/fasthttp"
    tracewayfasthttp "go.tracewayapp.com/tracewayfasthttp"
)

func main() {
    handler := func(ctx *fasthttp.RequestCtx) {
        ctx.SetStatusCode(200)
        ctx.SetBodyString("Hello, World!")
    }

    tracedHandler := tracewayfasthttp.New("${r}")(handler)
    fasthttp.ListenAndServe(":8080", tracedHandler)
}`;case"stdlib":return`package main

import (
    "net/http"

    tracewayhttp "go.tracewayapp.com/tracewayhttp"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", getUsers)

    handler := tracewayhttp.New("${r}")(mux)
    http.ListenAndServe(":8080", handler)
}`;case"react":return`import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider connectionString="${r}">
      <YourApp />
    </TracewayProvider>
  );
}

export default App;`;case"svelte":return`<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";
  import { browser } from "$app/environment";

  if (browser) {
    setupTraceway({
      connectionString: "${r}",
    });
  }
<\/script>

<slot />`;case"vuejs":return`import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  connectionString: "${r}",
}));

app.mount("#app");`;case"nextjs":return`// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <TracewayProvider connectionString="${r}">
      {children}
    </TracewayProvider>
  );
}

// app/layout.tsx
import { TracewayClientProvider } from "./traceway-provider";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html>
      <body>
        <TracewayClientProvider>{children}</TracewayClientProvider>
      </body>
    </html>
  );
}`;case"nestjs":return`import { Module } from "@nestjs/common";
import { TracewayModule } from "@tracewayapp/nest";

@Module({
    imports: [
        TracewayModule.forRoot({
            connectionString: "${r}",
        }),
    ],
})
export class AppModule {}`;case"express":return`import express from "express";
import { traceway } from "@tracewayapp/express";

const app = express();
app.use(traceway("${r}"));

app.get("/api/users", (req, res) => {
    res.json({ users: [] });
});

app.listen(8080);`;case"remix":return`import { withTraceway } from "@tracewayapp/remix";

export default withTraceway({
    connectionString: "${r}",
});`;case"jquery":return`import { init } from "@tracewayapp/jquery";

init("${r}");

// jQuery AJAX errors are captured automatically
// Distributed trace headers are injected into $.ajax() requests`;case"react-native":return`import { TracewayProvider } from "@tracewayapp/react-native";

export default function App() {
  return (
    <TracewayProvider connectionString="${r}">
      <RootNavigator />
    </TracewayProvider>
  );
}`;case"symfony":return`<?php
// public/index.php

use App\\Kernel;

require_once dirname(__DIR__) . '/vendor/autoload.php';

\\OpenTelemetry\\SDK\\SdkAutoloader::autoload();

// Fixes for Symfony's OTel auto-instrumentation:
// 1. Corrects http.route from internal route name to URL path template
// 2. Cleans up sub-request scopes so 500 error spans are exported
\\OpenTelemetry\\Instrumentation\\hook(
    \\Symfony\\Component\\HttpKernel\\HttpKernel::class,
    'handle',
    post: static function (
        \\Symfony\\Component\\HttpKernel\\HttpKernel $kernel,
        array $params,
        mixed $returnValue,
        ?\\Throwable $exception
    ): void {
        $request = ($params[0] instanceof \\Symfony\\Component\\HttpFoundation\\Request) ? $params[0] : null;
        if (null === $request) return;

        $type = $params[1] ?? \\Symfony\\Component\\HttpKernel\\HttpKernelInterface::MAIN_REQUEST;

        if ($type === \\Symfony\\Component\\HttpKernel\\HttpKernelInterface::SUB_REQUEST) {
            $scope = \\OpenTelemetry\\Context\\Context::storage()->scope();
            if (null !== $scope) {
                $span = \\OpenTelemetry\\API\\Trace\\Span::fromContext($scope->context());
                $scope->detach();
                $span->end();
            }
            return;
        }

        $routeParams = $request->attributes->get('_route_params', []);
        $path = $request->getPathInfo();
        if (\\is_array($routeParams)) {
            foreach ($routeParams as $name => $value) {
                if (\\is_string($value) && '' !== $value) {
                    $path = str_replace($value, '{' . $name . '}', $path);
                }
            }
        }

        $request->attributes->set('_route', $path);
    }
);

$kernel = new Kernel($_SERVER['APP_ENV'] ?? 'dev', (bool) ($_SERVER['APP_DEBUG'] ?? true));
$request = \\Symfony\\Component\\HttpFoundation\\Request::createFromGlobals();
$response = $kernel->handle($request);
$response->send();
$kernel->terminate($request, $response);`;case"laravel":return`<?php
// .env  - point the OTLP exporter at Traceway
//
// OTEL_SERVICE_NAME=my-laravel-app
// OTEL_TRACES_EXPORTER=otlp
// OTEL_METRICS_EXPORTER=otlp
// OTEL_LOGS_EXPORTER=otlp
// OTEL_EXPORTER_OTLP_PROTOCOL=http/json
// OTEL_EXPORTER_OTLP_ENDPOINT=${n}/api/otel
// OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${a||"YOUR_TOKEN"}"
//
// Optional: send Laravel logs to Traceway via the auto-injected 'otlp' channel
// LOG_CHANNEL=otlp

// That's it - keepsuit/laravel-opentelemetry's service provider auto-registers
// TraceRequestMiddleware as a global middleware, so every HTTP request, DB query,
// queued job, Redis call, cache op, view render and outbound Http:: call is
// traced automatically. Open config/opentelemetry.php to tune which
// instrumentations are enabled.`;case"django":return`# .env  - point the OTLP exporter at Traceway
#
# OTEL_SERVICE_NAME=my-django-app
# OTEL_TRACES_EXPORTER=otlp
# OTEL_METRICS_EXPORTER=otlp
# OTEL_LOGS_EXPORTER=otlp
# OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
# OTEL_EXPORTER_OTLP_ENDPOINT=${n}/api/otel
# OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20${a||"YOUR_TOKEN"}
# OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true

# Then launch Django through the OTel agent - no code changes needed:
#
#   opentelemetry-instrument python manage.py runserver
#   opentelemetry-instrument gunicorn myproject.wsgi:application
#
# DjangoInstrumentor auto-installs middleware at index 0 and traces every
# inbound request. opentelemetry-bootstrap also wired psycopg/redis/requests/
# celery/logging instrumentation, so DB queries, cache ops, outbound HTTP
# and queued tasks are traced automatically.`;case"hono":return"";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return`import 'package:flutter/material.dart';
import 'package:traceway/traceway.dart';

void main() {
  Traceway.run(
    connectionString: '${r}',
    options: TracewayOptions(
      screenCapture: true,
      version: '1.0.0',
    ),
    child: MyApp(),
  );
}`;case"android":return`import android.app.Application
import com.tracewayapp.traceway.Traceway
import com.tracewayapp.traceway.TracewayOptions

class MyApp : Application() {
    override fun onCreate() {
        super.onCreate()
        Traceway.init(
            application = this,
            connectionString = "${r}",
            options = TracewayOptions(version = "1.0.0"),
        )
    }
}`;case"ios":return`import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.start(
            connectionString: "${r}",
            options: TracewayOptions(version: "1.0.0")
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}`;default:return`package main

import (
    "go.tracewayapp.com"
)

func main() {
    traceway.Init(
        "${r}",
        traceway.WithVersion("1.0.0"),
        traceway.WithServerName("my-server"),
    )
}`}}function Rr(e){return e==="symfony"?`<?php
// src/Controller/TestController.php
namespace App\\Controller;

use Symfony\\Component\\HttpFoundation\\Response;
use Symfony\\Component\\Routing\\Attribute\\Route;

class TestController
{
    #[Route('/testing', name: 'testing')]
    public function index(): Response
    {
        throw new \\RuntimeException("Test error from Traceway integration");
    }
}`:e==="laravel"?`<?php
// routes/web.php
use Illuminate\\Support\\Facades\\Route;

Route::get('/testing', function () {
    throw new \\RuntimeException('Test error from Traceway integration');
});`:e==="django"?`# myapp/views.py
from django.http import HttpResponse


def testing(request):
    raise RuntimeError("Test error from Traceway integration")


# myproject/urls.py
from django.urls import path
from myapp import views

urlpatterns = [
    path("testing/", views.testing),
]`:e==="flutter"?`// Trigger a test error
throw StateError('Test error from Traceway integration');`:e==="android"?`// Trigger a test error
throw RuntimeException("Test error from Traceway integration")`:e==="ios"?`// Trigger a test error
fatalError("Test error from Traceway integration")`:e&&ke(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:`r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`}function Nr(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

TracewayClient.instance?.captureException(
  Exception('Test error'),
  StackTrace.current,
);`;if(e==="android")return`import com.tracewayapp.traceway.Traceway

try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e)
}`;if(e==="ios")return`import Traceway

do {
    try riskyOperation()
} catch {
    Traceway.capture(error)
}`;if(e&&ke(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

// In a component using the hook
const { captureException } = useTraceway();
captureException(new Error("Test error"));`;case"svelte":return`import { getTraceway } from "@tracewayapp/svelte";

const { captureException } = getTraceway();
captureException(new Error("Test error"));`;case"vuejs":return`import { useTraceway } from "@tracewayapp/vue";

const { captureException } = useTraceway();
captureException(new Error("Test error"));`;case"jquery":return`import { captureException } from "@tracewayapp/jquery";

captureException(new Error("Test error"));`;case"nextjs":return`import { useTraceway } from "@tracewayapp/react";

// In a client component
"use client";
const { captureException } = useTraceway();
captureException(new Error("Test error"));`;case"react-native":return`import { useTraceway } from "@tracewayapp/react-native";

// In a component using the hook
const { captureException } = useTraceway();
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${la(e)}";

captureException(new Error("Test error"));`}return`r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`}function la(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function hr(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android",ios:"iOS"}[e]||e}function Or(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||e==="ios"||ke(e)?"javascript":"go"}var da=x(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function xr(e,a){Oe(a,!0);const n=U(()=>fe.currentProject?.sourceMapToken??null),r=U(()=>Mt(a.backendUrl,a.token,t(n)));var l=da(),w=S(l),o=u(b(w),2),i=b(o);pe(i,{get code(){return Ct},get language(){return _e}}),E(o),E(w);var g=u(w,2),p=b(g),y=u(b(p),2),O=b(y);E(y),E(p);var d=u(p,2),f=b(d);It(f,{get parts(){return t(r)}}),E(d),E(g),ie(()=>oe(O,`Paste this prompt into your agent. Your instance URL and project token are already filled in${t(n)?", along with your source map upload token":""}.`)),s(e,l),xe()}const he="[A-Za-z$_][0-9A-Za-z$_]*",tt=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],at=["true","false","null","undefined","NaN","Infinity"],rt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],nt=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],ot=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],st=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],it=[].concat(ot,rt,nt);function ua(e){const a=e.regex,n=(c,{after:T})=>{const $="</"+c[0].slice(1);return c.input.indexOf($,T)!==-1},r=he,l={begin:"<>",end:"</>"},w=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,T)=>{const $=c[0].length+c.index,z=c.input[$];if(z==="<"||z===","){T.ignoreMatch();return}z===">"&&(n(c,{after:$})||T.ignoreMatch());let W;const q=c.input.substring($);if(W=q.match(/^\s*=/)){T.ignoreMatch();return}if((W=q.match(/^\s+extends\s+/))&&W.index===0){T.ignoreMatch();return}}},i={$pattern:he,keyword:tt,literal:at,built_in:it,"variable.language":st},g="[0-9](_?[0-9])*",p=`\\.(${g})`,y="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",O={className:"number",variants:[{begin:`(\\b(${y})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${y})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:i,contains:[]},f={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},A={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},L={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},D={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:r+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,f,A,_,L,{match:/\$\d+/},O];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:i,contains:["self"].concat(m)});const v=[].concat(D,d.contains),N=v.concat([{begin:/(\s*)\(/,end:/\)/,keywords:i,contains:["self"].concat(v)}]),R={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:N},G={variants:[{match:[/class/,/\s+/,r,/\s+/,/extends/,/\s+/,a.concat(r,"(",a.concat(/\./,r),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,r],scope:{1:"keyword",3:"title.class"}}]},I={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...rt,...nt]}},X={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,r,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[R],illegal:/%/},re={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(c){return a.concat("(?!",c.join("|"),")")}const K={match:a.concat(/\b/,ce([...ot,"super","import"].map(c=>`${c}\\s*\\(`)),r,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},M={begin:a.concat(/\./,a.lookahead(a.concat(r,/(?![0-9A-Za-z$_(])/))),end:r,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},h={match:[/get|set/,/\s+/,r,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},R]},P="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",k={match:[/const|var|let/,/\s+/,r,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(P)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[R]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:i,exports:{PARAMS_CONTAINS:N,CLASS_REFERENCE:I},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),X,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,f,A,_,L,D,{match:/\$\d+/},O,I,{scope:"attr",match:r+a.lookahead(":"),relevance:0},k,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[D,e.REGEXP_MODE,{className:"function",begin:P,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:N}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:w},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[R,e.inherit(e.TITLE_MODE,{begin:r,className:"title.function"})]},{match:/\.\.\./,relevance:0},M,{match:"\\$"+r,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[R]},K,re,G,h,{match:/\$[(.]/}]}}function pa(e){const a=e.regex,n=ua(e),r=he,l=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],w={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},o={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:l},contains:[n.exports.CLASS_REFERENCE]},i={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},g=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],p={$pattern:he,keyword:tt.concat(g),literal:at,built_in:it.concat(l),"variable.language":st},y={className:"meta",begin:"@"+r},O=(_,L,B)=>{const D=_.contains.findIndex(m=>m.label===L);if(D===-1)throw new Error("can not find mode to replace");_.contains.splice(D,1,B)};Object.assign(n.keywords,p),n.exports.PARAMS_CONTAINS.push(y);const d=n.contains.find(_=>_.scope==="attr"),f=Object.assign({},d,{match:a.concat(r,a.lookahead(/\s*\?:/))});n.exports.PARAMS_CONTAINS.push([n.exports.CLASS_REFERENCE,d,f]),n.contains=n.contains.concat([y,w,o,f]),O(n,"shebang",e.SHEBANG()),O(n,"use_strict",i);const A=n.contains.find(_=>_.label==="func.def");return A.relevance=0,Object.assign(n,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),n}const ct={name:"typescript",register:pa};function ma(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const ga={name:"gradle",register:ma};function _a(e){const a=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],n=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],r=["default","false","null","true"],l=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],w=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],o={keyword:l.concat(w),built_in:a,literal:r},i=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),g={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},p={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},y={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},O=e.inherit(y,{illegal:/\n/}),d={className:"subst",begin:/\{/,end:/\}/,keywords:o},f=e.inherit(d,{illegal:/\n/}),A={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,f]},_={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},d]},L=e.inherit(_,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},f]});d.contains=[_,A,y,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.C_BLOCK_COMMENT_MODE],f.contains=[L,A,O,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const B={variants:[p,_,A,y,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},D={begin:"<",end:">",contains:[{beginKeywords:"in out"},i]},m=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",v={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:o,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},B,g,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},i,D,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[i,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[i,D,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+m+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:o,contains:[{beginKeywords:n.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,D],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,relevance:0,contains:[B,g,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},v]}}const ba={name:"csharp",register:_a};function Ea(e){const a=e.regex,n="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",r=a.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),l=a.concat(r,/(::\w+)*/),o={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},i={className:"doctag",begin:"@[A-Za-z]+"},g={begin:"#<",end:">"},p=[e.COMMENT("#","$",{contains:[i]}),e.COMMENT("^=begin","^=end",{contains:[i],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],y={className:"subst",begin:/#\{/,end:/\}/,keywords:o},O={className:"string",contains:[e.BACKSLASH_ESCAPE,y],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:a.concat(/<<[-~]?'?/,a.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,y]})]}]},d="[1-9](_?[0-9])*|0",f="[0-9](_?[0-9])*",A={className:"number",relevance:0,variants:[{begin:`\\b(${d})(\\.(${f}))?([eE][+-]?(${f})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},_={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:o}]},R=[O,{variants:[{match:[/class\s+/,l,/\s+<\s+/,l]},{match:[/\b(class|module)\s+/,l]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:o},{match:[/(include|extend)\s+/,l],scope:{2:"title.class"},keywords:o},{relevance:0,match:[l,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:r,scope:"title.class"},{match:[/def/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[_]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[O,{begin:n}],relevance:0},A,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:o},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,y],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(g,p),relevance:0}].concat(g,p);y.contains=R,_.contains=R;const J=[{begin:/^\s*=>/,starts:{end:"$",contains:R}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:o,contains:R}}];return p.unshift(g),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:o,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(J).concat(p).concat(R)}}const fa={name:"ruby",register:Ea};function va(e){const a="true false yes no null",n="[\\w#;/?:@&=+$,.~*'()[\\]]+",r={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},l={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},w={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},o={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,l]},i=e.inherit(o,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),d={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},f={end:",",endsWithParent:!0,excludeEnd:!0,keywords:a,relevance:0},A={begin:/\{/,end:/\}/,contains:[f],illegal:"\\n",relevance:0},_={begin:"\\[",end:"\\]",contains:[f],illegal:"\\n",relevance:0},L=[r,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+n},{className:"type",begin:"!<"+n+">"},{className:"type",begin:"!"+n},{className:"type",begin:"!!"+n},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:a,keywords:{literal:a}},d,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},A,_,w,o],B=[...L];return B.pop(),B.push(i),f.contains=B,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:L}}const lt={name:"yaml",register:va};var ya=x("<!> Regenerate",1),Ta=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p> <!> <p class="mt-2 text-xs text-muted-foreground">This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code> instead of the flag.</p></div>`,1),Sa=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p> <!> <p class="mt-2 text-xs text-muted-foreground">Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.</p></div>`,1),wa=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p> <!> <p class="mt-2 text-xs text-muted-foreground">Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin
					embeds a ProGuard UUID into BuildConfig (matching Honeycomb's <code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping <code class="font-mono">&lt;uuid&gt;.txt</code>.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p> <!> <p class="mt-2 text-xs text-muted-foreground">Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native <code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so re-upload
					on each release.</p></div>`,1),Aa=x('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <!></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <!></div>',1),Ra=x('<!> <div><p class="mb-2 text-sm font-medium"> </p> <!></div>',1),Na=x('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!> <!></div></div> <!></div>'),ha=x('<p class="text-sm text-muted-foreground"> </p>'),Oa=x('<p class="text-sm text-muted-foreground">Plain release builds already report readable traces. Only obfuscated builds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code>) need this: generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code> after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/flutter" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Flutter docs</a></p>'),xa=x(`<p class="text-sm text-muted-foreground">Release crashes report against stripped machine code. Generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/ios" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">iOS docs</a></p>`),Ca=x(`<p class="text-sm text-muted-foreground">Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code> and native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries
				after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/symbolicator/android" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Android docs</a></p>`),Ia=x('<p class="text-sm text-muted-foreground"> </p>'),Ma=x("<!> Generating...",1),$a=x("<!> Generate Upload Token",1),La=x('<div class="flex items-center justify-between gap-4"><!> <!></div>'),Da=x("<!> <!>",1),Pa=x("<!> <!>",1),ka=x(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1),Ba=x("<!> <!>",1);function Ua(e,a){Oe(a,!0);const n={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:ct,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:Ne,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:Ne,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let r=Te("vite"),l=Te(!1);const w="npm install -D @tracewayapp/bundler-plugin",o=U(()=>fe.currentProject),i=U(()=>t(o)?.sourceMapToken??null),g=U(()=>Ve(t(o))),p=U(()=>t(o)?.framework==="flutter"),y=U(()=>t(o)?.framework==="ios"),O=U(()=>t(o)?.framework==="android"),d=U(()=>t(p)||t(y)||t(O)?"debug symbols":"source maps"),f=U(()=>t(o)?.framework!=="react-native"),A=U(()=>t(o)&&t(i)?`npx @tracewayapp/sourcemap-upload \\
  --url ${t(o).backendUrl} \\
  --token ${t(i)} \\
  --directory ${t(f)?n[t(r)].directory:"dist"}`:""),_="flutter build apk --release --obfuscate --split-debug-info=build/symbols",L=U(()=>t(o)&&t(i)?`dart run traceway:upload_symbols \\
  --token ${t(i)} \\
  --url ${t(o).backendUrl}`:""),B=`xcodebuild -scheme MyApp -configuration Release \\
  -archivePath build/MyApp.xcarchive archive`,D=U(()=>t(o)&&t(i)?`curl -X POST ${t(o).backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${t(i)}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`:""),m=U(()=>t(o)&&t(i)?`plugins {
  id("com.tracewayapp.symbols")
}

android {
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  token = "${t(i)}"
  url = "${t(o).backendUrl}"
}`:""),v="./gradlew assembleRelease uploadReleaseTracewaySymbols";let N=Te(!1);async function R(){me(l,!0);try{await fe.generateSourceMapToken()}finally{me(l,!1)}}async function G(){me(l,!0);try{await fe.generateSourceMapToken(),me(N,!1),Ot.success("Successfully regenerated the Upload Token")}finally{me(l,!1)}}var I=Ba(),X=S(I);{var J=K=>{var M=Na(),h=b(M),P=u(b(h),2),k=b(P),c=b(k,!0);E(k);var T=u(k,2);{let C=U(()=>t(i)??"");jt(T,{get text(){return t(C)}})}var $=u(T,2);Ae($,{variant:"destructiveOutline",size:"sm",onclick:()=>me(N,!0),children:(C,j)=>{var H=ya(),le=S(H);ea(le,{class:"mr-2 h-4 w-4"}),ee(),s(C,H)},$$slots:{default:!0}}),E(P),E(h);var z=u(h,2);{var W=C=>{var j=Ta(),H=S(j),le=u(b(H),2);pe(le,{code:_,get language(){return _e}}),ee(2),E(H);var ue=u(H,2),V=u(b(ue),2);pe(V,{get code(){return t(L)},get language(){return _e}}),ee(2),E(ue),s(C,j)},q=C=>{var j=te(),H=S(j);{var le=V=>{var F=Sa(),Q=S(F),de=u(b(Q),2);pe(de,{code:B,get language(){return _e}}),ee(2),E(Q);var se=u(Q,2),Z=u(b(se),2);pe(Z,{get code(){return t(D)},get language(){return _e}}),ee(2),E(se),s(V,F)},ue=V=>{var F=te(),Q=S(F);{var de=Z=>{var Y=wa(),ge=S(Y),Ee=u(b(ge),2);pe(Ee,{get code(){return t(m)},get language(){return Ne}}),ee(2),E(ge);var ve=u(ge,2),ye=u(b(ve),2);pe(ye,{code:v,get language(){return _e}}),ee(2),E(ve),s(Z,Y)},se=Z=>{var Y=Ra(),ge=S(Y);{var Ee=Ce=>{var Be=Aa(),Ie=S(Be),pt=u(b(Ie),2);pe(pt,{code:w,get language(){return _e}}),E(Ie);var Ue=u(Ie,2),Fe=u(b(Ue),2);ne(Fe,()=>Pe,(_t,bt)=>{bt(_t,{get value(){return t(r)},onValueChange:Se=>{Se&&me(r,Se,!0)},children:(Se,Qa)=>{var Ge=te(),Et=S(Ge);ne(Et,()=>Le,(ft,vt)=>{vt(ft,{class:"mb-2",children:(yt,Ja)=>{var Ke=te(),Tt=S(Ke);Re(Tt,17,()=>Object.entries(n),([$e,He])=>$e,($e,He)=>{var ze=U(()=>xt(t(He),2));let St=()=>t(ze)[0],wt=()=>t(ze)[1];var We=te(),At=S(We);ne(At,()=>De,(Rt,Nt)=>{Nt(Rt,{get value(){return St()},children:(ht,ja)=>{ee();var qe=be();ie(()=>oe(qe,wt().label)),s(ht,qe)},$$slots:{default:!0}})}),s($e,We)}),s(yt,Ke)},$$slots:{default:!0}})}),s(Se,Ge)},$$slots:{default:!0}})});var Me=u(Fe,2),mt=b(Me,!0);E(Me);var gt=u(Me,2);pe(gt,{get code(){return n[t(r)].code},get language(){return n[t(r)].language}}),E(Ue),ie(()=>oe(mt,n[t(r)].file)),s(Ce,Be)};ae(ge,Ce=>{t(f)&&Ce(Ee)})}var ve=u(ge,2),ye=b(ve),dt=b(ye,!0);E(ye);var ut=u(ye,2);pe(ut,{get code(){return t(A)},get language(){return _e}}),E(ve),ie(()=>oe(dt,t(f)?"Step 3: Upload after your production build":"Usage")),s(Z,Y)};ae(Q,Z=>{t(O)?Z(de):Z(se,!1)},!0)}s(V,F)};ae(H,V=>{t(y)?V(le):V(ue,!1)},!0)}s(C,j)};ae(z,C=>{t(p)?C(W):C(q,!1)})}E(M),ie(()=>oe(c,t(i))),s(K,M)},re=K=>{var M=te(),h=S(M);{var P=c=>{var T=ha(),$=b(T);E(T),ie(()=>oe($,`An upload token is required to upload ${t(d)??""}. Ask an organization admin to generate one
		from the Connection page.`)),s(c,T)},k=c=>{var T=La(),$=b(T);{var z=C=>{var j=Oa();s(C,j)},W=C=>{var j=te(),H=S(j);{var le=V=>{var F=xa();s(V,F)},ue=V=>{var F=te(),Q=S(F);{var de=Z=>{var Y=Ca();s(Z,Y)},se=Z=>{var Y=Ia(),ge=b(Y);E(Y),ie(()=>oe(ge,`Generate an upload token to start uploading ${t(d)??""} as part of your build process.`)),s(Z,Y)};ae(Q,Z=>{t(O)?Z(de):Z(se,!1)},!0)}s(V,F)};ae(H,V=>{t(y)?V(le):V(ue,!1)},!0)}s(C,j)};ae($,C=>{t(p)?C(z):C(W,!1)})}var q=u($,2);Ae(q,{variant:"outline",size:"sm",onclick:R,get disabled(){return t(l)},children:(C,j)=>{var H=te(),le=S(H);{var ue=F=>{var Q=Ma(),de=S(Q);qt(de,{class:"mr-2 h-4 w-4"}),ee(),s(F,Q)},V=F=>{var Q=$a(),de=S(Q);Qe(de,{class:"mr-2 h-4 w-4"}),ee(),s(F,Q)};ae(le,F=>{t(l)?F(ue):F(V,!1)})}s(C,H)},$$slots:{default:!0}}),E(T),s(c,T)};ae(h,c=>{t(g)?c(P):c(k,!1)},!0)}s(K,M)};ae(X,K=>{t(i)?K(J):K(re,!1)})}var ce=u(X,2);ne(ce,()=>Jt,(K,M)=>{M(K,{get open(){return t(N)},set open(h){me(N,h,!0)},children:(h,P)=>{var k=te(),c=S(k);ne(c,()=>Zt,(T,$)=>{$(T,{interactOutsideBehavior:"close",children:(z,W)=>{var q=ka(),C=S(q);ne(C,()=>Yt,(H,le)=>{le(H,{children:(ue,V)=>{var F=Da(),Q=S(F);ne(Q,()=>Xt,(se,Z)=>{Z(se,{children:(Y,ge)=>{ee();var Ee=be("Regenerate Upload Token");s(Y,Ee)},$$slots:{default:!0}})});var de=u(Q,2);ne(de,()=>Vt,(se,Z)=>{Z(se,{children:(Y,ge)=>{ee();var Ee=be(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);s(Y,Ee)},$$slots:{default:!0}})}),s(ue,F)},$$slots:{default:!0}})});var j=u(C,4);ne(j,()=>Qt,(H,le)=>{le(H,{class:"sm:justify-between",children:(ue,V)=>{var F=Pa(),Q=S(F);Ae(Q,{variant:"outline",onclick:()=>me(N,!1),get disabled(){return t(l)},children:(se,Z)=>{ee();var Y=be("Cancel");s(se,Y)},$$slots:{default:!0}});var de=u(Q,2);Ae(de,{variant:"destructive",onclick:G,get disabled(){return t(l)},children:(se,Z)=>{ee();var Y=be();ie(()=>oe(Y,t(l)?"Regenerating...":"Regenerate Token")),s(se,Y)},$$slots:{default:!0}}),s(ue,F)},$$slots:{default:!0}})}),s(z,q)},$$slots:{default:!0}})}),s(h,k)},$$slots:{default:!0}})}),s(e,I),xe()}var Fa=x("<!> ",1),Ga=x("<!> <!>",1),Ka=x("<!> <!>",1);function Ha(e,a){Oe(a,!0);let n=U(()=>fe.currentProject);const r=U(()=>t(n)?.framework==="flutter"),l=U(()=>t(n)?.framework==="ios"),w=U(()=>Ve(fe.currentProject));var o=te(),i=S(o);{var g=p=>{Gt(p,{children:(y,O)=>{var d=Ka(),f=S(d);zt(f,{children:(_,L)=>{var B=Ga(),D=S(B);Wt(D,{class:"flex items-center gap-2",children:(N,R)=>{var G=Fa(),I=S(G);Qe(I,{class:"h-5 w-5"});var X=u(I);ie(()=>oe(X,` ${t(r)||t(l)?"Symbol Upload":"Source Map Upload"}`)),s(N,G)},$$slots:{default:!0}});var m=u(D,2);{var v=N=>{Ht(N,{children:(R,G)=>{ee();var I=be(`Upload source maps to see original file names and line numbers in stack traces from
					minified code.`);s(R,I)},$$slots:{default:!0}})};ae(m,N=>{!t(r)&&!t(l)&&N(v)})}s(_,B)},$$slots:{default:!0}});var A=u(f,2);Kt(A,{children:(_,L)=>{Ua(_,{})},$$slots:{default:!0}}),s(y,d)},$$slots:{default:!0}})};ae(i,p=>{t(n)&&!t(w)&&p(g)})}s(e,o),xe()}var za=x('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),Wa=x('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),qa=x('<p class="pt-2 text-xs text-muted-foreground"><a> </a></p>'),Za=x('<div class="p-4"><!> <!></div>'),Ya=x('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),Xa=x('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function Cr(e,a){Oe(a,!0);let n=Te(Ze($t())),r=Te(Ze(Lt()));const l={bash:_e,go:Ft,javascript:Ne,typescript:ct,python:ca,gradle:ga,csharp:ba,ruby:fa,yaml:lt},w=U(()=>we.find(m=>m.id===t(n))??we[0]),o=U(()=>t(w).frameworks.find(m=>m.id===t(r))?.id??t(w).frameworks[0]?.id??""),i=U(()=>Dt(t(w).id,t(o),a.backendUrl,a.token));function g(m){const v=we.find(N=>N.id===m);v&&(me(n,v.id,!0),Pt(v.id))}function p(m){t(w).frameworks.some(v=>v.id===m)&&(me(r,m,!0),kt(m))}function y(m){return l[m??"bash"]}var O=Xa(),d=S(O),f=u(b(d),2);ne(f,()=>Pe,(m,v)=>{v(m,{get value(){return t(n)},onValueChange:g,children:(N,R)=>{var G=te(),I=S(G);ne(I,()=>Le,(X,J)=>{J(X,{class:"h-auto flex-wrap justify-start",children:(re,ce)=>{var K=te(),M=S(K);Re(M,17,()=>we,h=>h.id,(h,P)=>{var k=te(),c=S(k);ne(c,()=>De,(T,$)=>{$(T,{get value(){return t(P).id},children:(z,W)=>{ee();var q=be();ie(()=>oe(q,t(P).label)),s(z,q)},$$slots:{default:!0}})}),s(h,k)}),s(re,K)},$$slots:{default:!0}})}),s(N,G)},$$slots:{default:!0}})});var A=u(f,2);{var _=m=>{var v=za(),N=u(S(v),2);ne(N,()=>Pe,(R,G)=>{G(R,{get value(){return t(o)},onValueChange:p,children:(I,X)=>{var J=te(),re=S(J);ne(re,()=>Le,(ce,K)=>{K(ce,{class:"h-auto flex-wrap justify-start",children:(M,h)=>{var P=te(),k=S(P);Re(k,17,()=>t(w).frameworks,c=>c.id,(c,T)=>{var $=te(),z=S($);ne(z,()=>De,(W,q)=>{q(W,{get value(){return t(T).id},children:(C,j)=>{ee();var H=be();ie(()=>oe(H,t(T).label)),s(C,H)},$$slots:{default:!0}})}),s(c,$)}),s(M,P)},$$slots:{default:!0}})}),s(I,J)},$$slots:{default:!0}})}),s(m,v)};ae(A,m=>{t(w).frameworks.length>1&&m(_)})}E(d);var L=u(d,2);Re(L,19,()=>t(i),m=>t(w).id+t(o)+m.title,(m,v,N)=>{var R=Ya(),G=b(R),I=b(G),X=b(I),J=b(X,!0);E(X);var re=u(X,2),ce=b(re,!0);E(re),E(I);var K=u(I,2);{var M=k=>{var c=Wa(),T=b(c,!0);E(c),ie(()=>oe(T,t(v).description)),s(k,c)};ae(K,k=>{t(v).description&&k(M)})}E(G);var h=u(G,2);{var P=k=>{var c=Za(),T=b(c);{let W=U(()=>y(t(v).codeLanguage));pe(T,{get code(){return t(v).code},get language(){return t(W)}})}var $=u(T,2);{var z=W=>{var q=qa(),C=b(q);Bt(C,H=>({...H,target:"_blank",rel:"noopener noreferrer",class:"underline hover:text-foreground"}),[()=>({href:Ut(t(v).link.href)})]);var j=b(C,!0);E(C),E(q),ie(()=>oe(j,t(v).link.label)),s(W,q)};ae($,W=>{t(v).link&&W(z)})}E(c),s(k,c)};ae(h,k=>{t(v).code&&k(P)})}E(R),ie(()=>{oe(J,t(N)+1),oe(ce,t(v).title)}),s(m,R)});var B=u(L,2);{var D=m=>{Ha(m,{})};ae(B,m=>{t(n)==="nodejs"&&m(D)})}s(e,O),xe()}var Va=x('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function Ir(e,a){var n=Va(),r=b(n),l=u(b(r),4);Ye(l,{get value(){return a.endpoint}}),E(r);var w=u(r,2),o=u(b(w),2);Ye(o,{get value(){return a.authHeader}}),E(w);var i=u(w,2),g=u(b(i),2);pe(g,{get code(){return a.collectorConfig},get language(){return lt}}),E(i),E(n),s(e,n)}export{xr as A,Cr as O,Ha as S,Ir as a,ca as b,Ar as c,wr as d,Rr as e,Nr as f,hr as g,Or as h,Ne as j,Sr as p};
