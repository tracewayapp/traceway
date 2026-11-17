import{d as ke,p as fe,t as xt,h as Ve}from"./CrIPYF3_.js";import"./DsnmJJEf.js";import{p as he,e as x,f as S,i as u,h as E,r as f,g as t,u as B,t as ie,j as oe,d as s,a as xe,c as te,n as ee,l as be,m as me,q as Te,am as Ct,F as Ze,aa as Ye}from"./WpKdAexE.js";import{C as pe,b as _e,a as It,c as Mt,e as $t,f as Lt,h as Dt,O as Ae,i as Pt,s as kt,j as Bt}from"./DPJ9J6ea.js";import{i as ae}from"./BgSVxrKU.js";import{e as Ne}from"./BJsS8l-W.js";import{c as re}from"./CUxEhlM_.js";import{b as Ut,a as Ft}from"./Bzp-eEHB.js";import{r as Kt}from"./BZrl9jEp.js";import{T as Le,b as De,a as Pe}from"./DOeDumz9.js";import{g as Gt}from"./CiHBgu9W.js";import{C as Ht}from"./Fikm_kTV.js";import{C as zt}from"./BUf3WeMa.js";import{C as Wt}from"./Dwy-pTbi.js";import{C as qt,a as Zt}from"./B-5Ll-4D.js";import{B as Re}from"./CXvtDjrz.js";import{L as Yt}from"./CiJ5_dsh.js";import{A as Xt,a as Qt,b as Vt,d as Jt,c as jt,e as ea}from"./BOcndZjH.js";import{C as ta}from"./B_c9TUk3.js";import{R as aa}from"./Qi_mhhG_.js";import{R as na}from"./z63GUxh6.js";import{K as Je}from"./BD8Tg8bj.js";import{C as Xe}from"./BeBCh85q.js";const Qe="[A-Za-z$_][0-9A-Za-z$_]*",ra=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],oa=["true","false","null","undefined","NaN","Infinity"],je=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],et=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],tt=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],sa=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],ia=[].concat(tt,je,et);function ca(e){const a=e.regex,r=(c,{after:T})=>{const $="</"+c[0].slice(1);return c.input.indexOf($,T)!==-1},n=Qe,l={begin:"<>",end:"</>"},A=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,T)=>{const $=c[0].length+c.index,W=c.input[$];if(W==="<"||W===","){T.ignoreMatch();return}W===">"&&(r(c,{after:$})||T.ignoreMatch());let q;const F=c.input.substring($);if(q=F.match(/^\s*=/)){T.ignoreMatch();return}if((q=F.match(/^\s+extends\s+/))&&q.index===0){T.ignoreMatch();return}}},i={$pattern:Qe,keyword:ra,literal:oa,built_in:ia,"variable.language":sa},g="[0-9](_?[0-9])*",p=`\\.(${g})`,y="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",h={className:"number",variants:[{begin:`(\\b(${y})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${y})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:i,contains:[]},v={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},R={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},L={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},D={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:n+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,v,R,_,L,{match:/\$\d+/},h];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:i,contains:["self"].concat(m)});const b=[].concat(D,d.contains),w=b.concat([{begin:/(\s*)\(/,end:/\)/,keywords:i,contains:["self"].concat(b)}]),N={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:w},G={variants:[{match:[/class/,/\s+/,n,/\s+/,/extends/,/\s+/,a.concat(n,"(",a.concat(/\./,n),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,n],scope:{1:"keyword",3:"title.class"}}]},I={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...je,...et]}},X={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,n,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[N],illegal:/%/},ne={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(c){return a.concat("(?!",c.join("|"),")")}const H={match:a.concat(/\b/,ce([...tt,"super","import"].map(c=>`${c}\\s*\\(`)),n,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},M={begin:a.concat(/\./,a.lookahead(a.concat(n,/(?![0-9A-Za-z$_(])/))),end:n,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},O={match:[/get|set/,/\s+/,n,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},N]},k="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",P={match:[/const|var|let/,/\s+/,n,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(k)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[N]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:i,exports:{PARAMS_CONTAINS:w,CLASS_REFERENCE:I},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),X,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,v,R,_,L,D,{match:/\$\d+/},h,I,{scope:"attr",match:n+a.lookahead(":"),relevance:0},P,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[D,e.REGEXP_MODE,{className:"function",begin:k,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:w}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:A},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[N,e.inherit(e.TITLE_MODE,{begin:n,className:"title.function"})]},{match:/\.\.\./,relevance:0},M,{match:"\\$"+n,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[N]},H,ne,G,O,{match:/\$[(.]/}]}}const we={name:"javascript",register:ca};function la(e){const a=e.regex,r=/(?![A-Za-z0-9])(?![$])/,n=a.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,r),l=a.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,r),A=a.concat(/[A-Z]+/,r),o={scope:"variable",match:"\\$+"+n},i={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},g={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},p=e.inherit(e.APOS_STRING_MODE,{illegal:null}),y=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(g)}),h={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(g),"on:begin":(M,O)=>{O.data._beginMatch=M[1]||M[2]},"on:end":(M,O)=>{O.data._beginMatch!==M[1]&&O.ignoreMatch()}},d=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),v=`[ 	
]`,R={scope:"string",variants:[y,p,h,d]},_={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},L=["false","null","true"],U=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],D=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],b={keyword:U,literal:(M=>{const O=[];return M.forEach(k=>{O.push(k),k.toLowerCase()===k?O.push(k.toUpperCase()):O.push(k.toLowerCase())}),O})(L),built_in:D},w=M=>M.map(O=>O.replace(/\|\d+$/,"")),N={variants:[{match:[/new/,a.concat(v,"+"),a.concat("(?!",w(D).join("\\b|"),"\\b)"),l],scope:{1:"keyword",4:"title.class"}}]},G=a.concat(n,"\\b(?!\\()"),I={variants:[{match:[a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[l,a.concat(/::/,a.lookahead(/(?!class\b)/)),G],scope:{1:"title.class",3:"variable.constant"}},{match:[l,a.concat("::",a.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[l,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},X={scope:"attr",match:a.concat(n,a.lookahead(":"),a.lookahead(/(?!::)/))},J={relevance:0,begin:/\(/,end:/\)/,keywords:b,contains:[X,o,I,e.C_BLOCK_COMMENT_MODE,R,_,N]},ne={relevance:0,match:[/\b/,a.concat("(?!fn\\b|function\\b|",w(U).join("\\b|"),"|",w(D).join("\\b|"),"\\b)"),n,a.concat(v,"*"),a.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[J]};J.contains.push(ne);const ce=[X,I,e.C_BLOCK_COMMENT_MODE,R,_,N],H={begin:a.concat(/#\[\s*\\?/,a.either(l,A)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:L,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:L,keyword:["new","array"]},contains:["self",...ce]},...ce,{scope:"meta",variants:[{match:l},{match:A}]}]};return{case_insensitive:!1,keywords:b,contains:[H,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},i,{scope:"variable.language",match:/\$this\b/},o,ne,I,{match:[/const/,/\s/,n],scope:{1:"keyword",3:"variable.constant"}},N,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:b,contains:["self",H,o,I,e.C_BLOCK_COMMENT_MODE,R,_]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},R,_]}}const On={name:"php",register:la};function da(e){const a=e.regex,r=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),n=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],i={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:n,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},g={className:"meta",begin:/^(>>>|\.\.\.) /},p={className:"subst",begin:/\{/,end:/\}/,keywords:i,illegal:/#/},y={begin:/\{\{/,relevance:0},h={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,g,y,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,g,y,p]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,y,p]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,y,p]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},d="[0-9](_?[0-9])*",v=`(\\b(${d}))?\\.(${d})|\\b(${d})\\.`,R=`\\b|${n.join("|")}`,_={className:"number",relevance:0,variants:[{begin:`(\\b(${d})|(${v}))[eE][+-]?(${d})[jJ]?(?=${R})`},{begin:`(${v})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${R})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${R})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${R})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${R})`},{begin:`\\b(${d})[jJ](?=${R})`}]},L={className:"comment",begin:a.lookahead(/# type:/),end:/$/,keywords:i,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},U={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:["self",g,_,h,e.HASH_COMMENT_MODE]}]};return p.contains=[h,_,g],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:i,illegal:/(<\/|\?)|=>/,contains:[g,_,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},h,L,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,r],scope:{1:"keyword",3:"title.function"},contains:[U]},{variants:[{match:[/\bclass/,/\s+/,r,/\s*/,/\(\s*/,r,/\s*\)/]},{match:[/\bclass/,/\s+/,r]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[_,U,h]}]}}const ua={name:"python",register:da};function hn(e){switch(e){case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.1")';case"ios":return'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';default:return""}}function xn(e,a,r){const n=a?`${a}@${r}/api/report`:`YOUR_TOKEN@${r}/api/report`;switch(e){case"react":return`import { TracewayProvider } from "@tracewayapp/react";

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
  import { browser } from "$app/environment";

  if (browser) {
    setupTraceway({
      connectionString: "${n}",
    });
  }
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
}`;default:return""}}function Cn(e){return e==="symfony"?`<?php
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
throw new Error("Test error from Traceway integration");`:""}function In(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

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
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${pa(e)}";

captureException(new Error("Test error"));`}return""}function pa(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function Mn(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android",ios:"iOS"}[e]||e}function $n(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||e==="ios"||ke(e)?"javascript":"go"}var ma=x(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function Ln(e,a){he(a,!0);const r=B(()=>fe.currentProject?.sourceMapToken??null),n=B(()=>$t(a.backendUrl,a.token,t(r)));var l=ma(),A=S(l),o=u(E(A),2),i=E(o);pe(i,{get code(){return It},get language(){return _e}}),f(o),f(A);var g=u(A,2),p=E(g),y=u(E(p),2),h=E(y);f(y),f(p);var d=u(p,2),v=E(d);Mt(v,{get parts(){return t(n)}}),f(d),f(g),ie(()=>oe(h,`Paste this prompt into your agent. Your instance URL and project token are already filled in${t(r)?", along with your source map upload token":""}.`)),s(e,l),xe()}const Oe="[A-Za-z$_][0-9A-Za-z$_]*",at=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],nt=["true","false","null","undefined","NaN","Infinity"],rt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],ot=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],st=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],it=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],ct=[].concat(st,rt,ot);function ga(e){const a=e.regex,r=(c,{after:T})=>{const $="</"+c[0].slice(1);return c.input.indexOf($,T)!==-1},n=Oe,l={begin:"<>",end:"</>"},A=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(c,T)=>{const $=c[0].length+c.index,W=c.input[$];if(W==="<"||W===","){T.ignoreMatch();return}W===">"&&(r(c,{after:$})||T.ignoreMatch());let q;const F=c.input.substring($);if(q=F.match(/^\s*=/)){T.ignoreMatch();return}if((q=F.match(/^\s+extends\s+/))&&q.index===0){T.ignoreMatch();return}}},i={$pattern:Oe,keyword:at,literal:nt,built_in:ct,"variable.language":it},g="[0-9](_?[0-9])*",p=`\\.(${g})`,y="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",h={className:"number",variants:[{begin:`(\\b(${y})((${p})|\\.)?|(${p}))[eE][+-]?(${g})\\b`},{begin:`\\b(${y})\\b((${p})\\b|\\.)?|(${p})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},d={className:"subst",begin:"\\$\\{",end:"\\}",keywords:i,contains:[]},v={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"xml"}},R={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"css"}},_={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,d],subLanguage:"graphql"}},L={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,d]},D={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:n+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},m=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,v,R,_,L,{match:/\$\d+/},h];d.contains=m.concat({begin:/\{/,end:/\}/,keywords:i,contains:["self"].concat(m)});const b=[].concat(D,d.contains),w=b.concat([{begin:/(\s*)\(/,end:/\)/,keywords:i,contains:["self"].concat(b)}]),N={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:w},G={variants:[{match:[/class/,/\s+/,n,/\s+/,/extends/,/\s+/,a.concat(n,"(",a.concat(/\./,n),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,n],scope:{1:"keyword",3:"title.class"}}]},I={relevance:0,match:a.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...rt,...ot]}},X={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,n,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[N],illegal:/%/},ne={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(c){return a.concat("(?!",c.join("|"),")")}const H={match:a.concat(/\b/,ce([...st,"super","import"].map(c=>`${c}\\s*\\(`)),n,a.lookahead(/\s*\(/)),className:"title.function",relevance:0},M={begin:a.concat(/\./,a.lookahead(a.concat(n,/(?![0-9A-Za-z$_(])/))),end:n,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},O={match:[/get|set/,/\s+/,n,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},N]},k="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",P={match:[/const|var|let/,/\s+/,n,/\s*/,/=\s*/,/(async\s*)?/,a.lookahead(k)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[N]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:i,exports:{PARAMS_CONTAINS:w,CLASS_REFERENCE:I},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),X,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,v,R,_,L,D,{match:/\$\d+/},h,I,{scope:"attr",match:n+a.lookahead(":"),relevance:0},P,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[D,e.REGEXP_MODE,{className:"function",begin:k,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,contains:w}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:A},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[N,e.inherit(e.TITLE_MODE,{begin:n,className:"title.function"})]},{match:/\.\.\./,relevance:0},M,{match:"\\$"+n,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[N]},H,ne,G,O,{match:/\$[(.]/}]}}function _a(e){const a=e.regex,r=ga(e),n=Oe,l=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],A={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},o={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:l},contains:[r.exports.CLASS_REFERENCE]},i={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},g=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],p={$pattern:Oe,keyword:at.concat(g),literal:nt,built_in:ct.concat(l),"variable.language":it},y={className:"meta",begin:"@"+n},h=(_,L,U)=>{const D=_.contains.findIndex(m=>m.label===L);if(D===-1)throw new Error("can not find mode to replace");_.contains.splice(D,1,U)};Object.assign(r.keywords,p),r.exports.PARAMS_CONTAINS.push(y);const d=r.contains.find(_=>_.scope==="attr"),v=Object.assign({},d,{match:a.concat(n,a.lookahead(/\s*\?:/))});r.exports.PARAMS_CONTAINS.push([r.exports.CLASS_REFERENCE,d,v]),r.contains=r.contains.concat([y,A,o,v]),h(r,"shebang",e.SHEBANG()),h(r,"use_strict",i);const R=r.contains.find(_=>_.label==="func.def");return R.relevance=0,Object.assign(r,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),r}const lt={name:"typescript",register:_a};function ba(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const Ea={name:"gradle",register:ba};function fa(e){const a=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],r=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],n=["default","false","null","true"],l=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],A=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],o={keyword:l.concat(A),built_in:a,literal:n},i=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),g={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},p={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},y={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},h=e.inherit(y,{illegal:/\n/}),d={className:"subst",begin:/\{/,end:/\}/,keywords:o},v=e.inherit(d,{illegal:/\n/}),R={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,v]},_={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},d]},L=e.inherit(_,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},v]});d.contains=[_,R,y,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.C_BLOCK_COMMENT_MODE],v.contains=[L,R,h,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const U={variants:[p,_,R,y,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},D={begin:"<",end:">",contains:[{beginKeywords:"in out"},i]},m=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",b={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:o,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},U,g,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},i,D,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[i,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[i,D,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+m+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:o,contains:[{beginKeywords:r.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,D],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,relevance:0,contains:[U,g,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},b]}}const va={name:"csharp",register:fa};function ya(e){const a=e.regex,r="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",n=a.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),l=a.concat(n,/(::\w+)*/),o={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},i={className:"doctag",begin:"@[A-Za-z]+"},g={begin:"#<",end:">"},p=[e.COMMENT("#","$",{contains:[i]}),e.COMMENT("^=begin","^=end",{contains:[i],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],y={className:"subst",begin:/#\{/,end:/\}/,keywords:o},h={className:"string",contains:[e.BACKSLASH_ESCAPE,y],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:a.concat(/<<[-~]?'?/,a.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,y]})]}]},d="[1-9](_?[0-9])*|0",v="[0-9](_?[0-9])*",R={className:"number",relevance:0,variants:[{begin:`\\b(${d})(\\.(${v}))?([eE][+-]?(${v})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},_={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:o}]},N=[h,{variants:[{match:[/class\s+/,l,/\s+<\s+/,l]},{match:[/\b(class|module)\s+/,l]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:o},{match:[/(include|extend)\s+/,l],scope:{2:"title.class"},keywords:o},{relevance:0,match:[l,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:n,scope:"title.class"},{match:[/def/,/\s+/,r],scope:{1:"keyword",3:"title.function"},contains:[_]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[h,{begin:r}],relevance:0},R,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:o},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,y],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(g,p),relevance:0}].concat(g,p);y.contains=N,_.contains=N;const J=[{begin:/^\s*=>/,starts:{end:"$",contains:N}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:o,contains:N}}];return p.unshift(g),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:o,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(J).concat(p).concat(N)}}const Ta={name:"ruby",register:ya};function Sa(e){const a="true false yes no null",r="[\\w#;/?:@&=+$,.~*'()[\\]]+",n={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},l={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},A={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},o={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,l]},i=e.inherit(o,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),d={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},v={end:",",endsWithParent:!0,excludeEnd:!0,keywords:a,relevance:0},R={begin:/\{/,end:/\}/,contains:[v],illegal:"\\n",relevance:0},_={begin:"\\[",end:"\\]",contains:[v],illegal:"\\n",relevance:0},L=[n,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+r},{className:"type",begin:"!<"+r+">"},{className:"type",begin:"!"+r},{className:"type",begin:"!!"+r},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:a,keywords:{literal:a}},d,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},R,_,A,o],U=[...L];return U.pop(),U.push(i),v.contains=U,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:L}}const dt={name:"yaml",register:Sa};var Aa=x("<!> Regenerate",1),Ra=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p> <!> <p class="mt-2 text-xs text-muted-foreground">This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code> instead of the flag.</p></div>`,1),Na=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p> <!> <p class="mt-2 text-xs text-muted-foreground">Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.</p></div>`,1),wa=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p> <!> <p class="mt-2 text-xs text-muted-foreground">Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin
					embeds a ProGuard UUID into BuildConfig (matching Honeycomb's <code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping <code class="font-mono">&lt;uuid&gt;.txt</code>.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p> <!> <p class="mt-2 text-xs text-muted-foreground">Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native <code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so re-upload
					on each release.</p></div>`,1),Oa=x('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <!></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <!></div>',1),ha=x('<!> <div><p class="mb-2 text-sm font-medium"> </p> <!></div>',1),xa=x('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code> </code> <!> <!></div></div> <!></div>'),Ca=x('<p class="text-sm text-muted-foreground"> </p>'),Ia=x('<p class="text-sm text-muted-foreground">Plain release builds already report readable traces. Only obfuscated builds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code>) need this: generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code> after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/flutter" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Flutter docs</a></p>'),Ma=x(`<p class="text-sm text-muted-foreground">Release crashes report against stripped machine code. Generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/ios" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">iOS docs</a></p>`),$a=x(`<p class="text-sm text-muted-foreground">Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code> and native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries
				after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/symbolicator/android" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Android docs</a></p>`),La=x('<p class="text-sm text-muted-foreground"> </p>'),Da=x("<!> Generating...",1),Pa=x("<!> Generate Upload Token",1),ka=x('<div class="flex items-center justify-between gap-4"><!> <!></div>'),Ba=x("<!> <!>",1),Ua=x("<!> <!>",1),Fa=x(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1),Ka=x("<!> <!>",1);function Ga(e,a){he(a,!0);const r={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:lt,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:we,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:we,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let n=Te("vite"),l=Te(!1);const A="npm install -D @tracewayapp/bundler-plugin",o=B(()=>fe.currentProject),i=B(()=>t(o)?.sourceMapToken??null),g=B(()=>Ve(t(o))),p=B(()=>t(o)?.framework==="flutter"),y=B(()=>t(o)?.framework==="ios"),h=B(()=>t(o)?.framework==="android"),d=B(()=>t(p)||t(y)||t(h)?"debug symbols":"source maps"),v=B(()=>t(o)?.framework!=="react-native"),R=B(()=>t(o)&&t(i)?`npx @tracewayapp/sourcemap-upload \\
  --url ${t(o).backendUrl} \\
  --token ${t(i)} \\
  --directory ${t(v)?r[t(n)].directory:"dist"}`:""),_="flutter build apk --release --obfuscate --split-debug-info=build/symbols",L=B(()=>t(o)&&t(i)?`dart run traceway:upload_symbols \\
  --token ${t(i)} \\
  --url ${t(o).backendUrl}`:""),U=`xcodebuild -scheme MyApp -configuration Release \\
  -archivePath build/MyApp.xcarchive archive`,D=B(()=>t(o)&&t(i)?`curl -X POST ${t(o).backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${t(i)}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`:""),m=B(()=>t(o)&&t(i)?`plugins {
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
}`:""),b="./gradlew assembleRelease uploadReleaseTracewaySymbols";let w=Te(!1);async function N(){me(l,!0);try{await fe.generateSourceMapToken()}finally{me(l,!1)}}async function G(){me(l,!0);try{await fe.generateSourceMapToken(),me(w,!1),xt.success("Successfully regenerated the Upload Token")}finally{me(l,!1)}}var I=Ka(),X=S(I);{var J=H=>{var M=xa(),O=E(M),k=u(E(O),2),P=E(k),c=E(P,!0);f(P);var T=u(P,2);{let C=B(()=>t(i)??"");ta(T,{get text(){return t(C)}})}var $=u(T,2);Re($,{variant:"destructiveOutline",size:"sm",onclick:()=>me(w,!0),children:(C,j)=>{var z=Aa(),le=S(z);na(le,{class:"mr-2 h-4 w-4"}),ee(),s(C,z)},$$slots:{default:!0}}),f(k),f(O);var W=u(O,2);{var q=C=>{var j=Ra(),z=S(j),le=u(E(z),2);pe(le,{code:_,get language(){return _e}}),ee(2),f(z);var ue=u(z,2),Q=u(E(ue),2);pe(Q,{get code(){return t(L)},get language(){return _e},secret:!0}),ee(2),f(ue),s(C,j)},F=C=>{var j=te(),z=S(j);{var le=Q=>{var K=Na(),V=S(K),de=u(E(V),2);pe(de,{code:U,get language(){return _e}}),ee(2),f(V);var se=u(V,2),Z=u(E(se),2);pe(Z,{get code(){return t(D)},get language(){return _e},secret:!0}),ee(2),f(se),s(Q,K)},ue=Q=>{var K=te(),V=S(K);{var de=Z=>{var Y=wa(),ge=S(Y),Ee=u(E(ge),2);pe(Ee,{get code(){return t(m)},get language(){return we},secret:!0}),ee(2),f(ge);var ve=u(ge,2),ye=u(E(ve),2);pe(ye,{code:b,get language(){return _e}}),ee(2),f(ve),s(Z,Y)},se=Z=>{var Y=ha(),ge=S(Y);{var Ee=Ce=>{var Be=Oa(),Ie=S(Be),mt=u(E(Ie),2);pe(mt,{code:A,get language(){return _e}}),f(Ie);var Ue=u(Ie,2),Fe=u(E(Ue),2);re(Fe,()=>Pe,(bt,Et)=>{Et(bt,{get value(){return t(n)},onValueChange:Se=>{Se&&me(n,Se,!0)},children:(Se,en)=>{var Ke=te(),ft=S(Ke);re(ft,()=>Le,(vt,yt)=>{yt(vt,{class:"mb-2",children:(Tt,tn)=>{var Ge=te(),St=S(Ge);Ne(St,17,()=>Object.entries(r),([$e,He])=>$e,($e,He)=>{var ze=B(()=>Ct(t(He),2));let At=()=>t(ze)[0],Rt=()=>t(ze)[1];var We=te(),Nt=S(We);re(Nt,()=>De,(wt,Ot)=>{Ot(wt,{get value(){return At()},children:(ht,an)=>{ee();var qe=be();ie(()=>oe(qe,Rt().label)),s(ht,qe)},$$slots:{default:!0}})}),s($e,We)}),s(Tt,Ge)},$$slots:{default:!0}})}),s(Se,Ke)},$$slots:{default:!0}})});var Me=u(Fe,2),gt=E(Me,!0);f(Me);var _t=u(Me,2);pe(_t,{get code(){return r[t(n)].code},get language(){return r[t(n)].language}}),f(Ue),ie(()=>oe(gt,r[t(n)].file)),s(Ce,Be)};ae(ge,Ce=>{t(v)&&Ce(Ee)})}var ve=u(ge,2),ye=E(ve),ut=E(ye,!0);f(ye);var pt=u(ye,2);pe(pt,{get code(){return t(R)},get language(){return _e},secret:!0}),f(ve),ie(()=>oe(ut,t(v)?"Step 3: Upload after your production build":"Usage")),s(Z,Y)};ae(V,Z=>{t(h)?Z(de):Z(se,!1)},!0)}s(Q,K)};ae(z,Q=>{t(y)?Q(le):Q(ue,!1)},!0)}s(C,j)};ae(W,C=>{t(p)?C(q):C(F,!1)})}f(M),ie(()=>{Ut(P,1,`flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all ${aa}`),oe(c,t(i))}),s(H,M)},ne=H=>{var M=te(),O=S(M);{var k=c=>{var T=Ca(),$=E(T);f(T),ie(()=>oe($,`An upload token is required to upload ${t(d)??""}. Ask an organization admin to generate one
		from the Connection page.`)),s(c,T)},P=c=>{var T=ka(),$=E(T);{var W=C=>{var j=Ia();s(C,j)},q=C=>{var j=te(),z=S(j);{var le=Q=>{var K=Ma();s(Q,K)},ue=Q=>{var K=te(),V=S(K);{var de=Z=>{var Y=$a();s(Z,Y)},se=Z=>{var Y=La(),ge=E(Y);f(Y),ie(()=>oe(ge,`Generate an upload token to start uploading ${t(d)??""} as part of your build process.`)),s(Z,Y)};ae(V,Z=>{t(h)?Z(de):Z(se,!1)},!0)}s(Q,K)};ae(z,Q=>{t(y)?Q(le):Q(ue,!1)},!0)}s(C,j)};ae($,C=>{t(p)?C(W):C(q,!1)})}var F=u($,2);Re(F,{variant:"outline",size:"sm",onclick:N,get disabled(){return t(l)},children:(C,j)=>{var z=te(),le=S(z);{var ue=K=>{var V=Da(),de=S(V);Yt(de,{class:"mr-2 h-4 w-4"}),ee(),s(K,V)},Q=K=>{var V=Pa(),de=S(V);Je(de,{class:"mr-2 h-4 w-4"}),ee(),s(K,V)};ae(le,K=>{t(l)?K(ue):K(Q,!1)})}s(C,z)},$$slots:{default:!0}}),f(T),s(c,T)};ae(O,c=>{t(g)?c(k):c(P,!1)},!0)}s(H,M)};ae(X,H=>{t(i)?H(J):H(ne,!1)})}var ce=u(X,2);re(ce,()=>ea,(H,M)=>{M(H,{get open(){return t(w)},set open(O){me(w,O,!0)},children:(O,k)=>{var P=te(),c=S(P);re(c,()=>Xt,(T,$)=>{$(T,{interactOutsideBehavior:"close",children:(W,q)=>{var F=Fa(),C=S(F);re(C,()=>Qt,(z,le)=>{le(z,{children:(ue,Q)=>{var K=Ba(),V=S(K);re(V,()=>Vt,(se,Z)=>{Z(se,{children:(Y,ge)=>{ee();var Ee=be("Regenerate Upload Token");s(Y,Ee)},$$slots:{default:!0}})});var de=u(V,2);re(de,()=>Jt,(se,Z)=>{Z(se,{children:(Y,ge)=>{ee();var Ee=be(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);s(Y,Ee)},$$slots:{default:!0}})}),s(ue,K)},$$slots:{default:!0}})});var j=u(C,4);re(j,()=>jt,(z,le)=>{le(z,{class:"sm:justify-between",children:(ue,Q)=>{var K=Ua(),V=S(K);Re(V,{variant:"outline",onclick:()=>me(w,!1),get disabled(){return t(l)},children:(se,Z)=>{ee();var Y=be("Cancel");s(se,Y)},$$slots:{default:!0}});var de=u(V,2);Re(de,{variant:"destructive",onclick:G,get disabled(){return t(l)},children:(se,Z)=>{ee();var Y=be();ie(()=>oe(Y,t(l)?"Regenerating...":"Regenerate Token")),s(se,Y)},$$slots:{default:!0}}),s(ue,K)},$$slots:{default:!0}})}),s(W,F)},$$slots:{default:!0}})}),s(O,P)},$$slots:{default:!0}})}),s(e,I),xe()}var Ha=x("<!> ",1),za=x("<!> <!>",1),Wa=x("<!> <!>",1);function qa(e,a){he(a,!0);let r=B(()=>fe.currentProject);const n=B(()=>t(r)?.framework==="flutter"),l=B(()=>t(r)?.framework==="ios"),A=B(()=>Ve(fe.currentProject));var o=te(),i=S(o);{var g=p=>{Ht(p,{children:(y,h)=>{var d=Wa(),v=S(d);qt(v,{children:(_,L)=>{var U=za(),D=S(U);Zt(D,{class:"flex items-center gap-2",children:(w,N)=>{var G=Ha(),I=S(G);Je(I,{class:"h-5 w-5"});var X=u(I);ie(()=>oe(X,` ${t(n)||t(l)?"Symbol Upload":"Source Map Upload"}`)),s(w,G)},$$slots:{default:!0}});var m=u(D,2);{var b=w=>{Wt(w,{children:(N,G)=>{ee();var I=be(`Upload source maps to see original file names and line numbers in stack traces from
					minified code.`);s(N,I)},$$slots:{default:!0}})};ae(m,w=>{!t(n)&&!t(l)&&w(b)})}s(_,U)},$$slots:{default:!0}});var R=u(v,2);zt(R,{children:(_,L)=>{Ga(_,{})},$$slots:{default:!0}}),s(y,d)},$$slots:{default:!0}})};ae(i,p=>{t(r)&&!t(A)&&p(g)})}s(e,o),xe()}var Za=x('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),Ya=x('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),Xa=x('<p class="pt-2 text-xs text-muted-foreground"><a> </a></p>'),Qa=x('<div class="p-4"><!> <!></div>'),Va=x('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),Ja=x('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function Dn(e,a){he(a,!0);let r=Te(Ze(Ye(()=>a.initialTarget??Lt()))),n=Te(Ze(Ye(()=>a.initialTarget?a.initialFramework??null:Dt())));const l={bash:_e,go:Gt,javascript:we,typescript:lt,python:ua,gradle:Ea,csharp:va,ruby:Ta,yaml:dt},A=B(()=>Ae.find(m=>m.id===t(r))??Ae[0]),o=B(()=>t(A).frameworks.find(m=>m.id===t(n))?.id??t(A).frameworks[0]?.id??""),i=B(()=>Pt(t(A).id,t(o),a.backendUrl,a.token));function g(m){const b=Ae.find(w=>w.id===m);b&&(me(r,b.id,!0),kt(b.id))}function p(m){t(A).frameworks.some(b=>b.id===m)&&(me(n,m,!0),Bt(m))}function y(m){return l[m??"bash"]}var h=Ja(),d=S(h),v=u(E(d),2);re(v,()=>Pe,(m,b)=>{b(m,{get value(){return t(r)},onValueChange:g,children:(w,N)=>{var G=te(),I=S(G);re(I,()=>Le,(X,J)=>{J(X,{class:"h-auto flex-wrap justify-start",children:(ne,ce)=>{var H=te(),M=S(H);Ne(M,17,()=>Ae,O=>O.id,(O,k)=>{var P=te(),c=S(P);re(c,()=>De,(T,$)=>{$(T,{get value(){return t(k).id},children:(W,q)=>{ee();var F=be();ie(()=>oe(F,t(k).label)),s(W,F)},$$slots:{default:!0}})}),s(O,P)}),s(ne,H)},$$slots:{default:!0}})}),s(w,G)},$$slots:{default:!0}})});var R=u(v,2);{var _=m=>{var b=Za(),w=u(S(b),2);re(w,()=>Pe,(N,G)=>{G(N,{get value(){return t(o)},onValueChange:p,children:(I,X)=>{var J=te(),ne=S(J);re(ne,()=>Le,(ce,H)=>{H(ce,{class:"h-auto flex-wrap justify-start",children:(M,O)=>{var k=te(),P=S(k);Ne(P,17,()=>t(A).frameworks,c=>c.id,(c,T)=>{var $=te(),W=S($);re(W,()=>De,(q,F)=>{F(q,{get value(){return t(T).id},children:(C,j)=>{ee();var z=be();ie(()=>oe(z,t(T).label)),s(C,z)},$$slots:{default:!0}})}),s(c,$)}),s(M,k)},$$slots:{default:!0}})}),s(I,J)},$$slots:{default:!0}})}),s(m,b)};ae(R,m=>{t(A).frameworks.length>1&&m(_)})}f(d);var L=u(d,2);Ne(L,19,()=>t(i),m=>t(A).id+t(o)+m.title,(m,b,w)=>{var N=Va(),G=E(N),I=E(G),X=E(I),J=E(X,!0);f(X);var ne=u(X,2),ce=E(ne,!0);f(ne),f(I);var H=u(I,2);{var M=P=>{var c=Ya(),T=E(c,!0);f(c),ie(()=>oe(T,t(b).description)),s(P,c)};ae(H,P=>{t(b).description&&P(M)})}f(G);var O=u(G,2);{var k=P=>{var c=Qa(),T=E(c);{let q=B(()=>y(t(b).codeLanguage)),F=B(()=>t(b).code.includes(a.token));pe(T,{get code(){return t(b).code},get language(){return t(q)},get secret(){return t(F)}})}var $=u(T,2);{var W=q=>{var F=Xa(),C=E(F);Ft(C,z=>({...z,target:"_blank",rel:"noopener noreferrer",class:"underline hover:text-foreground"}),[()=>({href:Kt(t(b).link.href)})]);var j=E(C,!0);f(C),f(F),ie(()=>oe(j,t(b).link.label)),s(q,F)};ae($,q=>{t(b).link&&q(W)})}f(c),s(P,c)};ae(O,P=>{t(b).code&&P(k)})}f(N),ie(()=>{oe(J,t(w)+1),oe(ce,t(b).title)}),s(m,N)});var U=u(L,2);{var D=m=>{qa(m,{})};ae(U,m=>{t(r)==="nodejs"&&m(D)})}s(e,h),xe()}var ja=x('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function Pn(e,a){var r=ja(),n=E(r),l=u(E(n),4);Xe(l,{get value(){return a.endpoint}}),f(n);var A=u(n,2),o=u(E(A),2);Xe(o,{get value(){return a.authHeader},secret:!0}),f(A);var i=u(A,2),g=u(E(i),2);pe(g,{get code(){return a.collectorConfig},get language(){return dt},secret:!0}),f(i),f(r),s(e,r)}export{Ln as A,Dn as O,qa as S,Pn as a,ua as b,xn as c,hn as d,Cn as e,In as f,Mn as g,$n as h,we as j,On as p};
