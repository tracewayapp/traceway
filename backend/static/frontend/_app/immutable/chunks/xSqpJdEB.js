import{d as De,p as Ee,j as Ve,t as Ot}from"./CnaJn3K1.js";import"./DsnmJJEf.js";import{p as he,f as h,k as u,o as I,r as M,v as ue,g as t,l as ne,m as j,e as i,a as xe,u as U,h as k,j as se,n as X,t as be,i as le,s as Te,av as wt,b as We,ak as Ze}from"./Dpw_JtvN.js";import{C as oe,b as _e,a as ht,c as xt,e as Ct,f as It,h as Mt,O as Ae,i as $t,s as Lt,j as Dt}from"./RVtV3Jyv.js";import{i as ie}from"./B-sWumEd.js";import{e as Ne}from"./C30FIux9.js";import{c as J}from"./6DFM7H3j.js";import{a as Pt,b as kt}from"./DsDJ8mJz.js";import{r as Bt}from"./CPffSbUj.js";import{a as Me,b as $e,T as Le}from"./DLA73nR-.js";import{g as Ut}from"./CiHBgu9W.js";import{C as Ft}from"./0LdSvTf8.js";import{C as Kt}from"./VcB9Plnc.js";import{C as Gt}from"./btz5vfNi.js";import{C as Ht,a as zt}from"./iFT2k972.js";import{B as Re}from"./DxyobRN5.js";import{L as Wt}from"./ZPphWhc0.js";import{A as Zt,a as qt,b as Yt,c as Xt,d as Qt,e as Vt}from"./BSkuWdjN.js";import{C as Jt}from"./ksTLuZ2x.js";import{R as jt}from"./Qi_mhhG_.js";import{R as ea}from"./oTx0x2Sh.js";import{K as Je}from"./BDxH_Yr8.js";import{C as qe}from"./DduCjjCo.js";const Ye="[A-Za-z$_][0-9A-Za-z$_]*",ta=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],aa=["true","false","null","undefined","NaN","Infinity"],je=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],et=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],tt=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],na=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],ra=[].concat(tt,je,et);function oa(e){const a=e.regex,r=(c,{after:x})=>{const B="</"+c[0].slice(1);return c.input.indexOf(B,x)!==-1},n=Ye,l={begin:"<>",end:"</>"},T=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,x)=>{const B=c[0].length+c.index,W=c.input[B];if(W==="<"||W===","){x.ignoreMatch();return}W===">"&&(r(c,{after:B})||x.ignoreMatch());let y;const C=c.input.substring(B);if(y=C.match(/^\s*=/)){x.ignoreMatch();return}if((y=C.match(/^\s+extends\s+/))&&y.index===0){x.ignoreMatch();return}}},s={$pattern:Ye,keyword:ta,literal:aa,built_in:ra,"variable.language":na},g="[0-9](_?[0-9])*",p=`\\.(${g})`,f="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",O={className:"number",variants:[{begin:`(\\b(${f})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${f})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:s,contains:[]},E={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},S={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},D={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},P={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:n+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,E,S,_,D,{match:/\$\d+/},O];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:s,contains:["self"].concat(m)});const b=[].concat(P,d.contains),R=b.concat([{begin:/(\s*)\(/,end:/\)/,keywords:s,contains:["self"].concat(b)}]),A={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:s,contains:R},G={variants:[{match:[/class/,/\s+/,n,/\s+/,/extends/,/\s+/,a.concat(n,"(",a.concat(/\./,n),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,n],scope:{1:"keyword",3:"title.class"}}]},$={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...je,...et]}},Z={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},q={variants:[{match:[/function/,/\s+/,n,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[A],illegal:/%/},ee={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function re(c){return a.concat("(?!",c.join("|"),")")}const Q={match:a.concat(/\b/,re([...tt,"super","import"].map(c=>`${c}\\s*\\(`)),n,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},w={begin:a.concat(/\./,a.lookahead(a.concat(n,/(?![0-9A-Za-z$_(])/))),end:n,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},v={match:[/get|set/,/\s+/,n,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},A]},N="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",H={match:[/const|var|let/,/\s+/,n,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(N)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[A]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:s,exports:{PARAMS_CONTAINS:R,CLASS_REFERENCE:$},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),Z,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,E,S,_,D,P,{match:/\$\d+/},O,$,{scope:"attr",match:n+a.lookahead(":"),relevance:0},H,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[P,e.REGEXP_MODE,{className:"function",begin:N,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:s,contains:R}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:T},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},q,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[A,e.inherit(e.TITLE_MODE,{begin:n,className:"title.function"})]},{match:/\.\.\./,relevance:0},w,{match:"\\$"+n,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[A]},Q,ee,G,v,{match:/\$[(.]/}]}}const Oe={name:"javascript",register:oa};function sa(e){const a=e.regex,r=/(?![A-Za-z0-9])(?![$])/,n=a.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,r),l=a.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,r),T=a.concat(/[A-Z]+/,r),o={scope:"variable",match:"\\$+"+n},s={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},g={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},p=e.inherit(e.APOS_STRING_MODE,{illegal:null}),f=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(g)}),O={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(g),"on:begin":(w,v)=>{v.data._beginMatch=w[1]||w[2]},"on:end":(w,v)=>{v.data._beginMatch!==w[1]&&v.ignoreMatch()}},d=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),E=`[ 	
]`,S={scope:"string",variants:[f,p,O,d]},_={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},D=["false","null","true"],F=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],P=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],b={keyword:F,literal:(w=>{const v=[];return w.forEach(N=>{v.push(N),N.toLowerCase()===N?v.push(N.toUpperCase()):v.push(N.toLowerCase())}),v})(D),built_in:P},R=w=>w.map(v=>v.replace(/\|\d+$/,"")),A={variants:[{match:[/new/,a.concat(E,"+"),a.concat("(?!",R(P).join("\\b|"),"\\b)"),l],scope:{1:"keyword",4:"title.class"}}]},G=a.concat(n,"\\b(?!\\()"),$={variants:[{match:[a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[l,a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{1:"title.class",3:"variable.constant"}},{match:[l,a.concat("::",a.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[l,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},Z={scope:"attr",match:a.concat(n,a.lookahead(":"),a.lookahead(/(?!::)/))},q={relevance:0,begin:/\(/,end:/\)/,keywords:b,contains:[Z,o,$,e.C_BLOCK_COMMENT_MODE,S,_,A]},ee={relevance:0,match:[/\b/,a.concat("(?!fn\\b|function\\b|",R(F).join("\\b|"),"|",R(P).join("\\b|"),"\\b)"),n,a.concat(E,"*"),a.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[q]};q.contains.push(ee);const re=[Z,$,e.C_BLOCK_COMMENT_MODE,S,_,A],Q={begin:a.concat(/#\[\s*\\?/,a.either(l,T)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:D,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:D,keyword:["new","array"]},contains:["self",...re]},...re,{scope:"meta",variants:[{match:l},{match:T}]}]};return{case_insensitive:!1,keywords:b,contains:[Q,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},s,{scope:"variable.language",match:/\$this\b/},o,ee,$,{match:[/const/,/\s/,n],scope:{1:"keyword",3:"variable.constant"}},A,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:b,contains:["self",Q,o,$,e.C_BLOCK_COMMENT_MODE,S,_]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},S,_]}}const En={name:"php",register:sa};function ia(e){const a=e.regex,r=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),n=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],s={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:n,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},g={className:"meta",begin:/^(>>>|\.\.\.) /},p={className:"subst",begin:/\{/,end:/\}/,keywords:s,illegal:/#/},f={begin:/\{\{/,relevance:0},O={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g,f,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g,f,p]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,f,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,f,p]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},d="[0-9](_?[0-9])*",E=`(\\b(${d}))?\\.(${d})|\\b(${d})\\.`,S=`\\b|${n.join("|")}`,_={className:"number",relevance:0,variants:[{begin:`(\\b(${d})|(${E}))[eE][+-]?(${d})[jJ]?(?=${S})`},{begin:`(${E})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${S})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${S})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${S})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${S})`},{begin:`\\b(${d})[jJ](?=${S})`}]},D={className:"comment",begin:a.lookahead(/# type:/),end:/$/,keywords:s,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},F={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:s,contains:["self",g,_,O,e.HASH_COMMENT_MODE]}]};return p.contains=[O,_,g],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:s,illegal:/(<\/|\?)|=>/,contains:[g,_,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},O,D,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,r],scope:{1:"keyword",3:"title.function"},contains:[F]},{variants:[{match:[/\bclass/,/\s+/,r,/\s*/,/\(\s*/,r,/\s*\)/]},{match:[/\bclass/,/\s+/,r]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[_,F,O]}]}}const ca={name:"python",register:ia};function fn(e){switch(e){case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.1")';case"ios":return'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';default:return""}}function vn(e,a,r){const n=a?`${a}@${r}/api/report`:`YOUR_TOKEN@${r}/api/report`;switch(e){case"react":return`import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider connectionString="${n}">
      <YourApp />
    </TracewayProvider>
  );
}

export default App;`;case"svelte":return`<!-- src/routes/+layout.svelte -->
<script>
  import { setupTraceway } from "@tracewayapp/svelte";

  setupTraceway({
    connectionString: "${n}",
  });
<\/script>

<slot />`;case"vuejs":return`import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  connectionString: "${n}",
}));

app.mount("#app");`;case"nextjs":return`// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <TracewayProvider connectionString="${n}">
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
            connectionString: "${n}",
        }),
    ],
})
export class AppModule {}`;case"express":return`import express from "express";
import { traceway } from "@tracewayapp/express";

const app = express();
app.use(traceway("${n}"));

app.get("/api/users", (req, res) => {
    res.json({ users: [] });
});

app.listen(8080);`;case"remix":return`import { withTraceway } from "@tracewayapp/remix";

export default withTraceway({
    connectionString: "${n}",
});`;case"jquery":return`import { init } from "@tracewayapp/jquery";

init("${n}");

// jQuery AJAX errors are captured automatically
// Distributed trace headers are injected into $.ajax() requests`;case"react-native":return`import { TracewayProvider } from "@tracewayapp/react-native";

export default function App() {
  return (
    <TracewayProvider connectionString="${n}">
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
// OTEL_EXPORTER_OTLP_ENDPOINT=${r}/api/otel
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
# OTEL_EXPORTER_OTLP_ENDPOINT=${r}/api/otel
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
    connectionString: '${n}',
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
            connectionString = "${n}",
            options = TracewayOptions(version = "1.0.0"),
        )
    }
}`;case"ios":return`import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.start(
            connectionString: "${n}",
            options: TracewayOptions(version: "1.0.0")
        )
    }

    var body: some Scene {
        WindowGroup {
            ContentView()
        }
    }
}`;default:return""}}function yn(e){return e==="symfony"?`<?php
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
fatalError("Test error from Traceway integration")`:e&&De(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:""}function Tn(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

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
}`;if(e&&De(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

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

captureException(new Error("Test error"));`}return""}function la(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function Sn(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android",ios:"iOS"}[e]||e}function An(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||e==="ios"||De(e)?"javascript":"go"}var da=k(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function Rn(e,a){he(a,!0);const r=U(()=>Ee.currentProject?.sourceMapToken??null),n=U(()=>Ct(a.backendUrl,a.token,t(r)));var l=da(),T=h(l),o=u(I(T),2),s=I(o);oe(s,{get code(){return ht},get language(){return _e}}),M(o),M(T);var g=u(T,2),p=I(g),f=u(I(p),2),O=ue(f);M(p);var d=u(p,2),E=I(d);xt(E,{get parts(){return t(n)}}),M(d),M(g),ne(()=>j(O,`Paste this prompt into your agent. Your instance URL and project token are already filled in${t(r)?", along with your source map upload token":""}.`)),i(e,l),xe()}const we="[A-Za-z$_][0-9A-Za-z$_]*",at=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],nt=["true","false","null","undefined","NaN","Infinity"],rt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],ot=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],st=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],it=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],ct=[].concat(st,rt,ot);function ua(e){const a=e.regex,r=(c,{after:x})=>{const B="</"+c[0].slice(1);return c.input.indexOf(B,x)!==-1},n=we,l={begin:"<>",end:"</>"},T=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,x)=>{const B=c[0].length+c.index,W=c.input[B];if(W==="<"||W===","){x.ignoreMatch();return}W===">"&&(r(c,{after:B})||x.ignoreMatch());let y;const C=c.input.substring(B);if(y=C.match(/^\s*=/)){x.ignoreMatch();return}if((y=C.match(/^\s+extends\s+/))&&y.index===0){x.ignoreMatch();return}}},s={$pattern:we,keyword:at,literal:nt,built_in:ct,"variable.language":it},g="[0-9](_?[0-9])*",p=`\\.(${g})`,f="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",O={className:"number",variants:[{begin:`(\\b(${f})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${f})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:s,contains:[]},E={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},S={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},D={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},P={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:n+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,E,S,_,D,{match:/\$\d+/},O];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:s,contains:["self"].concat(m)});const b=[].concat(P,d.contains),R=b.concat([{begin:/(\s*)\(/,end:/\)/,keywords:s,contains:["self"].concat(b)}]),A={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:s,contains:R},G={variants:[{match:[/class/,/\s+/,n,/\s+/,/extends/,/\s+/,a.concat(n,"(",a.concat(/\./,n),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,n],scope:{1:"keyword",3:"title.class"}}]},$={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...rt,...ot]}},Z={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},q={variants:[{match:[/function/,/\s+/,n,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[A],illegal:/%/},ee={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function re(c){return a.concat("(?!",c.join("|"),")")}const Q={match:a.concat(/\b/,re([...st,"super","import"].map(c=>`${c}\\s*\\(`)),n,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},w={begin:a.concat(/\./,a.lookahead(a.concat(n,/(?![0-9A-Za-z$_(])/))),end:n,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},v={match:[/get|set/,/\s+/,n,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},A]},N="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",H={match:[/const|var|let/,/\s+/,n,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(N)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[A]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:s,exports:{PARAMS_CONTAINS:R,CLASS_REFERENCE:$},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),Z,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,E,S,_,D,P,{match:/\$\d+/},O,$,{scope:"attr",match:n+a.lookahead(":"),relevance:0},H,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[P,e.REGEXP_MODE,{className:"function",begin:N,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:s,contains:R}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:T},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},q,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[A,e.inherit(e.TITLE_MODE,{begin:n,className:"title.function"})]},{match:/\.\.\./,relevance:0},w,{match:"\\$"+n,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[A]},Q,ee,G,v,{match:/\$[(.]/}]}}function pa(e){const a=e.regex,r=ua(e),n=we,l=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],T={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},o={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:l},contains:[r.exports.CLASS_REFERENCE]},s={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},g=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],p={$pattern:we,keyword:at.concat(g),literal:nt,built_in:ct.concat(l),"variable.language":it},f={className:"meta",begin:"@"+n},O=(_,D,F)=>{const P=_.contains.findIndex(m=>m.label===D);if(P===-1)throw new Error("can not find mode to replace");_.contains.splice(P,1,F)};Object.assign(r.keywords,p),r.exports.PARAMS_CONTAINS.push(f);const d=r.contains.find(_=>_.scope==="attr"),E=Object.assign({},d,{match:a.concat(n,a.lookahead(/\s*\?:/))});r.exports.PARAMS_CONTAINS.push([r.exports.CLASS_REFERENCE,d,E]),r.contains=r.contains.concat([f,T,o,E]),O(r,"shebang",e.SHEBANG()),O(r,"use_strict",s);const S=r.contains.find(_=>_.label==="func.def");return S.relevance=0,Object.assign(r,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),r}const lt={name:"typescript",register:pa};function ma(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const ga={name:"gradle",register:ma};function _a(e){const a=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],r=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],n=["default","false","null","true"],l=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],T=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],o={keyword:l.concat(T),built_in:a,literal:n},s=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),g={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},p={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},f={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},O=e.inherit(f,{illegal:/\n/}),d={className:"subst",begin:/\{/,end:/\}/,keywords:o},E=e.inherit(d,{illegal:/\n/}),S={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,E]},_={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},d]},D=e.inherit(_,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},E]});d.contains=[_,S,f,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.C_BLOCK_COMMENT_MODE],E.contains=[D,S,O,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const F={variants:[p,_,S,f,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},P={begin:"<",end:">",contains:[{beginKeywords:"in out"},s]},m=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",b={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:o,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},F,g,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},s,P,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[s,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[s,P,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+m+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:o,contains:[{beginKeywords:r.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,P],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,relevance:0,contains:[F,g,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},b]}}const ba={name:"csharp",register:_a};function Ea(e){const a=e.regex,r="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",n=a.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),l=a.concat(n,/(::\w+)*/),o={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},s={className:"doctag",begin:"@[A-Za-z]+"},g={begin:"#<",end:">"},p=[e.COMMENT("#","$",{contains:[s]}),e.COMMENT("^=begin","^=end",{contains:[s],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],f={className:"subst",begin:/#\{/,end:/\}/,keywords:o},O={className:"string",contains:[e.BACKSLASH_ESCAPE,f],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:a.concat(/<<[-~]?'?/,a.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,f]})]}]},d="[1-9](_?[0-9])*|0",E="[0-9](_?[0-9])*",S={className:"number",relevance:0,variants:[{begin:`\\b(${d})(\\.(${E}))?([eE][+-]?(${E})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},_={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:o}]},A=[O,{variants:[{match:[/class\s+/,l,/\s+<\s+/,l]},{match:[/\b(class|module)\s+/,l]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:o},{match:[/(include|extend)\s+/,l],scope:{2:"title.class"},keywords:o},{relevance:0,match:[l,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:n,scope:"title.class"},{match:[/def/,/\s+/,r],scope:{1:"keyword",3:"title.function"},contains:[_]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[O,{begin:r}],relevance:0},S,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:o},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,f],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(g,p),relevance:0}].concat(g,p);f.contains=A,_.contains=A;const q=[{begin:/^\s*=>/,starts:{end:"$",contains:A}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:o,contains:A}}];return p.unshift(g),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:o,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(q).concat(p).concat(A)}}const fa={name:"ruby",register:Ea};function va(e){const a="true false yes no null",r="[\\w#;/?:@&=+$,.~*'()[\\]]+",n={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},l={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},T={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},o={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,l]},s=e.inherit(o,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),d={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},E={end:",",endsWithParent:!0,excludeEnd:!0,keywords:a,relevance:0},S={begin:/\{/,end:/\}/,contains:[E],illegal:"\\n",relevance:0},_={begin:"\\[",end:"\\]",contains:[E],illegal:"\\n",relevance:0},D=[n,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+r},{className:"type",begin:"!<"+r+">"},{className:"type",begin:"!"+r},{className:"type",begin:"!!"+r},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:a,keywords:{literal:a}},d,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},S,_,T,o],F=[...D];return F.pop(),F.push(s),E.contains=F,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:D}}const dt={name:"yaml",register:va};var ya=k("<!> Regenerate",1),Ta=k(`<div><p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p> <!> <p class="mt-2 text-xs text-muted-foreground">This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code> instead of the flag.</p></div>`,1),Sa=k(`<div><p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p> <!> <p class="mt-2 text-xs text-muted-foreground">Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.</p></div>`,1),Aa=k(`<div><p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p> <!> <p class="mt-2 text-xs text-muted-foreground">Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin
					embeds a ProGuard UUID into BuildConfig (matching Honeycomb's <code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping <code class="font-mono">&lt;uuid&gt;.txt</code>.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p> <!> <p class="mt-2 text-xs text-muted-foreground">Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native <code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so re-upload
					on each release.</p></div>`,1),Ra=k('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <!></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <!></div>',1),Na=k('<!> <div><p class="mb-2 text-sm font-medium"> </p> <!></div>',1),Oa=k('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code> </code> <!> <!></div></div> <!></div>'),Xe=k('<p class="text-sm text-muted-foreground"> </p>'),wa=k('<p class="text-sm text-muted-foreground">Plain release builds already report readable traces. Only obfuscated builds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code>) need this: generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code> after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/flutter" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Flutter docs</a></p>'),ha=k(`<p class="text-sm text-muted-foreground">Release crashes report against stripped machine code. Generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/ios" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">iOS docs</a></p>`),xa=k(`<p class="text-sm text-muted-foreground">Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code> and native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries
				after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/symbolicator/android" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Android docs</a></p>`),Ca=k("<!> Generating...",1),Ia=k("<!> Generate Upload Token",1),Ma=k('<div class="flex items-center justify-between gap-4"><!> <!></div>'),Ie=k("<!> <!>",1),$a=k(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1);function La(e,a){he(a,!0);const r={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:lt,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:Oe,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:Oe,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let n=Te("vite"),l=Te(!1);const T="npm install -D @tracewayapp/bundler-plugin",o=U(()=>Ee.currentProject),s=U(()=>t(o)?.sourceMapToken??null),g=U(()=>Ve(t(o))),p=U(()=>t(o)?.framework==="flutter"),f=U(()=>t(o)?.framework==="ios"),O=U(()=>t(o)?.framework==="android"),d=U(()=>t(p)||t(f)||t(O)?"debug symbols":"source maps"),E=U(()=>t(o)?.framework!=="react-native"),S=U(()=>t(o)&&t(s)?`npx @tracewayapp/sourcemap-upload \\
  --url ${t(o).backendUrl} \\
  --token ${t(s)} \\
  --directory ${t(E)?r[t(n)].directory:"dist"}`:""),_="flutter build apk --release --obfuscate --split-debug-info=build/symbols",D=U(()=>t(o)&&t(s)?`dart run traceway:upload_symbols \\
  --token ${t(s)} \\
  --url ${t(o).backendUrl}`:""),F=`xcodebuild -scheme MyApp -configuration Release \\
  -archivePath build/MyApp.xcarchive archive`,P=U(()=>t(o)&&t(s)?`curl -X POST ${t(o).backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${t(s)}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`:""),m=U(()=>t(o)&&t(s)?`plugins {
  id("com.tracewayapp.symbols")
}

android {
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  token = "${t(s)}"
  url = "${t(o).backendUrl}"
}`:""),b="./gradlew assembleRelease uploadReleaseTracewaySymbols";let R=Te(!1);async function A(){le(l,!0);try{await Ee.generateSourceMapToken()}finally{le(l,!1)}}async function G(){le(l,!0);try{await Ee.generateSourceMapToken(),le(R,!1),Ot.success("Successfully regenerated the Upload Token")}finally{le(l,!1)}}var $=Ie(),Z=h($);{var q=w=>{var v=Oa(),N=I(v),H=u(I(N),2),c=I(H),x=ue(c,!0),B=u(c,2);{let z=U(()=>t(s)??"");Jt(B,{get text(){return t(z)}})}var W=u(B,2);Re(W,{variant:"destructiveOutline",size:"sm",onclick:()=>le(R,!0),children:(z,K)=>{var L=ya(),V=h(L);ea(V,{class:"mr-2 h-4 w-4"}),X(),i(z,L)},$$slots:{default:!0}}),M(H),M(N);var y=u(N,2);{var C=z=>{var K=Ta(),L=h(K),V=u(I(L),2);oe(V,{code:_,get language(){return _e}}),X(2),M(L);var Y=u(L,2),ae=u(I(Y),2);oe(ae,{get code(){return t(D)},get language(){return _e},secret:!0}),X(2),M(Y),i(z,K)},te=z=>{var K=Sa(),L=h(K),V=u(I(L),2);oe(V,{code:F,get language(){return _e}}),X(2),M(L);var Y=u(L,2),ae=u(I(Y),2);oe(ae,{get code(){return t(P)},get language(){return _e},secret:!0}),X(2),M(Y),i(z,K)},pe=z=>{var K=Aa(),L=h(K),V=u(I(L),2);oe(V,{get code(){return t(m)},get language(){return Oe},secret:!0}),X(2),M(L);var Y=u(L,2),ae=u(I(Y),2);oe(ae,{code:b,get language(){return _e}}),X(2),M(Y),i(z,K)},de=z=>{var K=Na(),L=h(K);{var V=ge=>{var ce=Ra(),ve=h(ce),ye=u(I(ve),2);oe(ye,{code:T,get language(){return _e}}),M(ve);var Pe=u(ve,2),ke=u(I(Pe),2);J(ke,()=>Le,(mt,gt)=>{gt(mt,{get value(){return t(n)},onValueChange:Se=>{Se&&le(n,Se,!0)},children:(Se,za)=>{var Ue=se(),_t=h(Ue);J(_t,()=>Me,(bt,Et)=>{Et(bt,{class:"mb-2",children:(ft,Wa)=>{var Fe=se(),vt=h(Fe);Ne(vt,17,()=>Object.entries(r),([Ce,Ke])=>Ce,(Ce,Ke)=>{var Ge=U(()=>wt(t(Ke),2));let yt=()=>t(Ge)[0],Tt=()=>t(Ge)[1];var He=se(),St=h(He);J(St,()=>$e,(At,Rt)=>{Rt(At,{get value(){return yt()},children:(Nt,Za)=>{X();var ze=be();ne(()=>j(ze,Tt().label)),i(Nt,ze)},$$slots:{default:!0}})}),i(Ce,He)}),i(ft,Fe)},$$slots:{default:!0}})}),i(Se,Ue)},$$slots:{default:!0}})});var Be=u(ke,2),ut=ue(Be,!0),pt=u(Be,2);oe(pt,{get code(){return r[t(n)].code},get language(){return r[t(n)].language}}),M(Pe),ne(()=>j(ut,r[t(n)].file)),i(ge,ce)};ie(L,ge=>{t(E)&&ge(V)})}var Y=u(L,2),ae=I(Y),fe=ue(ae,!0),me=u(ae,2);oe(me,{get code(){return t(S)},get language(){return _e},secret:!0}),M(Y),ne(()=>j(fe,t(E)?"Step 3: Upload after your production build":"Usage")),i(z,K)};ie(y,z=>{t(p)?z(C):t(f)?z(te,1):t(O)?z(pe,2):z(de,-1)})}M(v),ne(()=>{Pt(c,1,`flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all ${jt}`),j(x,t(s))}),i(w,v)},ee=w=>{var v=Xe(),N=ue(v);ne(()=>j(N,`An upload token is required to upload ${t(d)??""}. Ask an organization admin to generate one
		from the Connection page.`)),i(w,v)},re=w=>{var v=Ma(),N=I(v);{var H=y=>{var C=wa();i(y,C)},c=y=>{var C=ha();i(y,C)},x=y=>{var C=xa();i(y,C)},B=y=>{var C=Xe(),te=ue(C);ne(()=>j(te,`Generate an upload token to start uploading ${t(d)??""} as part of your build process.`)),i(y,C)};ie(N,y=>{t(p)?y(H):t(f)?y(c,1):t(O)?y(x,2):y(B,-1)})}var W=u(N,2);Re(W,{variant:"outline",size:"sm",onclick:A,get disabled(){return t(l)},children:(y,C)=>{var te=se(),pe=h(te);{var de=K=>{var L=Ca(),V=h(L);Wt(V,{class:"mr-2 h-4 w-4"}),X(),i(K,L)},z=K=>{var L=Ia(),V=h(L);Je(V,{class:"mr-2 h-4 w-4"}),X(),i(K,L)};ie(pe,K=>{t(l)?K(de):K(z,-1)})}i(y,te)},$$slots:{default:!0}}),M(v),i(w,v)};ie(Z,w=>{t(s)?w(q):t(g)?w(ee,1):w(re,-1)})}var Q=u(Z,2);J(Q,()=>Vt,(w,v)=>{v(w,{get open(){return t(R)},set open(N){le(R,N,!0)},children:(N,H)=>{var c=se(),x=h(c);J(x,()=>Zt,(B,W)=>{W(B,{interactOutsideBehavior:"close",children:(y,C)=>{var te=$a(),pe=h(te);J(pe,()=>qt,(z,K)=>{K(z,{children:(L,V)=>{var Y=Ie(),ae=h(Y);J(ae,()=>Yt,(me,ge)=>{ge(me,{children:(ce,ve)=>{X();var ye=be("Regenerate Upload Token");i(ce,ye)},$$slots:{default:!0}})});var fe=u(ae,2);J(fe,()=>Xt,(me,ge)=>{ge(me,{children:(ce,ve)=>{X();var ye=be(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);i(ce,ye)},$$slots:{default:!0}})}),i(L,Y)},$$slots:{default:!0}})});var de=u(pe,4);J(de,()=>Qt,(z,K)=>{K(z,{class:"sm:justify-between",children:(L,V)=>{var Y=Ie(),ae=h(Y);Re(ae,{variant:"outline",onclick:()=>le(R,!1),get disabled(){return t(l)},children:(me,ge)=>{X();var ce=be("Cancel");i(me,ce)},$$slots:{default:!0}});var fe=u(ae,2);Re(fe,{variant:"destructive",onclick:G,get disabled(){return t(l)},children:(me,ge)=>{X();var ce=be();ne(()=>j(ce,t(l)?"Regenerating...":"Regenerate Token")),i(me,ce)},$$slots:{default:!0}}),i(L,Y)},$$slots:{default:!0}})}),i(y,te)},$$slots:{default:!0}})}),i(N,c)},$$slots:{default:!0}})}),i(e,$),xe()}var Da=k("<!> ",1),Qe=k("<!> <!>",1);function Pa(e,a){he(a,!0);let r=U(()=>Ee.currentProject);const n=U(()=>t(r)?.framework==="flutter"),l=U(()=>t(r)?.framework==="ios"),T=U(()=>Ve(Ee.currentProject));var o=se(),s=h(o);{var g=p=>{Ft(p,{children:(f,O)=>{var d=Qe(),E=h(d);Ht(E,{children:(_,D)=>{var F=Qe(),P=h(F);zt(P,{class:"flex items-center gap-2",children:(R,A)=>{var G=Da(),$=h(G);Je($,{class:"h-5 w-5"});var Z=u($);ne(()=>j(Z,` ${t(n)||t(l)?"Symbol Upload":"Source Map Upload"}`)),i(R,G)},$$slots:{default:!0}});var m=u(P,2);{var b=R=>{Gt(R,{children:(A,G)=>{X();var $=be(`Upload source maps to see original file names and line numbers in stack traces from
					minified code.`);i(A,$)},$$slots:{default:!0}})};ie(m,R=>{!t(n)&&!t(l)&&R(b)})}i(_,F)},$$slots:{default:!0}});var S=u(E,2);Kt(S,{children:(_,D)=>{La(_,{})},$$slots:{default:!0}}),i(f,d)},$$slots:{default:!0}})};ie(s,p=>{t(r)&&!t(T)&&p(g)})}i(e,o),xe()}var ka=k('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),Ba=k('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),Ua=k('<p class="pt-2 text-xs text-muted-foreground"><a> </a></p>'),Fa=k('<div class="p-4"><!> <!></div>'),Ka=k('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),Ga=k('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function Nn(e,a){he(a,!0);let r=Te(We(Ze(()=>a.initialTarget??It()))),n=Te(We(Ze(()=>a.initialTarget?a.initialFramework??null:Mt())));const l={bash:_e,go:Ut,javascript:Oe,typescript:lt,python:ca,gradle:ga,csharp:ba,ruby:fa,yaml:dt},T=U(()=>Ae.find(m=>m.id===t(r))??Ae[0]),o=U(()=>t(T).frameworks.find(m=>m.id===t(n))?.id??t(T).frameworks[0]?.id??""),s=U(()=>$t(t(T).id,t(o),a.backendUrl,a.token));function g(m){const b=Ae.find(R=>R.id===m);b&&(le(r,b.id,!0),Lt(b.id))}function p(m){t(T).frameworks.some(b=>b.id===m)&&(le(n,m,!0),Dt(m))}function f(m){return l[m??"bash"]}var O=Ga(),d=h(O),E=u(I(d),2);J(E,()=>Le,(m,b)=>{b(m,{get value(){return t(r)},onValueChange:g,children:(R,A)=>{var G=se(),$=h(G);J($,()=>Me,(Z,q)=>{q(Z,{class:"h-auto flex-wrap justify-start",children:(ee,re)=>{var Q=se(),w=h(Q);Ne(w,17,()=>Ae,v=>v.id,(v,N)=>{var H=se(),c=h(H);J(c,()=>$e,(x,B)=>{B(x,{get value(){return t(N).id},children:(W,y)=>{X();var C=be();ne(()=>j(C,t(N).label)),i(W,C)},$$slots:{default:!0}})}),i(v,H)}),i(ee,Q)},$$slots:{default:!0}})}),i(R,G)},$$slots:{default:!0}})});var S=u(E,2);{var _=m=>{var b=ka(),R=u(h(b),2);J(R,()=>Le,(A,G)=>{G(A,{get value(){return t(o)},onValueChange:p,children:($,Z)=>{var q=se(),ee=h(q);J(ee,()=>Me,(re,Q)=>{Q(re,{class:"h-auto flex-wrap justify-start",children:(w,v)=>{var N=se(),H=h(N);Ne(H,17,()=>t(T).frameworks,c=>c.id,(c,x)=>{var B=se(),W=h(B);J(W,()=>$e,(y,C)=>{C(y,{get value(){return t(x).id},children:(te,pe)=>{X();var de=be();ne(()=>j(de,t(x).label)),i(te,de)},$$slots:{default:!0}})}),i(c,B)}),i(w,N)},$$slots:{default:!0}})}),i($,q)},$$slots:{default:!0}})}),i(m,b)};ie(S,m=>{t(T).frameworks.length>1&&m(_)})}M(d);var D=u(d,2);Ne(D,19,()=>t(s),m=>t(T).id+t(o)+m.title,(m,b,R)=>{var A=Ka(),G=I(A),$=I(G),Z=I($),q=ue(Z,!0),ee=u(Z,2),re=ue(ee,!0);M($);var Q=u($,2);{var w=H=>{var c=Ba(),x=ue(c,!0);ne(()=>j(x,t(b).description)),i(H,c)};ie(Q,H=>{t(b).description&&H(w)})}M(G);var v=u(G,2);{var N=H=>{var c=Fa(),x=I(c);{let y=U(()=>f(t(b).codeLanguage)),C=U(()=>t(b).code.includes(a.token));oe(x,{get code(){return t(b).code},get language(){return t(y)},get secret(){return t(C)}})}var B=u(x,2);{var W=y=>{var C=Ua(),te=I(C);kt(te,de=>({...de,target:"_blank",rel:"noopener noreferrer",class:"underline hover:text-foreground"}),[()=>({href:Bt(t(b).link.href)})]);var pe=ue(te,!0);M(C),ne(()=>j(pe,t(b).link.label)),i(y,C)};ie(B,y=>{t(b).link&&y(W)})}M(c),i(H,c)};ie(v,H=>{t(b).code&&H(N)})}M(A),ne(()=>{j(q,t(R)+1),j(re,t(b).title)}),i(m,A)});var F=u(D,2);{var P=m=>{Pa(m,{})};ie(F,m=>{t(r)==="nodejs"&&m(P)})}i(e,O),xe()}var Ha=k('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function On(e,a){var r=Ha(),n=I(r),l=u(I(n),4);qe(l,{get value(){return a.endpoint}}),M(n);var T=u(n,2),o=u(I(T),2);qe(o,{get value(){return a.authHeader},secret:!0}),M(T);var s=u(T,2),g=u(I(s),2);oe(g,{get code(){return a.collectorConfig},get language(){return dt},secret:!0}),M(s),M(r),i(e,r)}export{Rn as A,Nn as O,Pa as S,On as a,ca as b,vn as c,fn as d,yn as e,Tn as f,Sn as g,An as h,Oe as j,En as p};
