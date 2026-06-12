import{i as Xe,p as xe,t as kt,b as ct}from"./BDNSRZSr.js";import{l as Dt,s as Bt,p as Ut,i as Q}from"./7a4wz9Fg.js";import{d as F,c as E,b as s,p as Se,f as N,n as U,t as ge,s as A,a as Ae,e as T,g as r,h as ue,r as y,k as le,i as k,l as re,u as oe,x as Ft,v as rt}from"./Bq244wU7.js";import{c as V}from"./npWQtwSL.js";import{a as Be,b as Me,T as Ue}from"./UP4zC35H.js";import{e as $e,i as Kt}from"./CMOe-Whj.js";import{B as me}from"./D_PvveBf.js";import{C as Te}from"./CLfv_bqz.js";import{C as we}from"./v3DbPef4.js";import{a as Pe,s as Gt}from"./BXRhph22.js";import{H as ke,g as zt}from"./Cw-wiYxU.js";import{C as Ht,a as qt,b as Wt}from"./BobgzAwU.js";import{C as Zt}from"./DrJw9jKm.js";import{C as Yt}from"./ohiL75jn.js";import{L as Xt}from"./Cvq7tg_y.js";import{A as Vt,a as Qt,b as Jt,c as jt,d as ea,e as ta}from"./CNJx9-5U.js";import{t as De}from"./_rwR96iw.js";import{R as aa}from"./Bse7pcGf.js";import{I as na,s as ra}from"./B9OYPylh.js";function lt(e,t){const n=Dt(t,["children","$$slots","$$events","$$legacy"]);const a=[["path",{d:"M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"}],["circle",{cx:"16.5",cy:"7.5",r:".5",fill:"currentColor"}]];na(e,Bt({name:"key-round"},()=>n,{get iconNode(){return a},children:(o,d)=>{var i=F(),l=E(i);ra(l,t,"default",{},null),s(o,i)},$$slots:{default:!0}}))}const ot="[A-Za-z$_][0-9A-Za-z$_]*",oa=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],sa=["true","false","null","undefined","NaN","Infinity"],dt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],ut=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],pt=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],ia=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],ca=[].concat(pt,dt,ut);function la(e){const t=e.regex,n=(S,{after:M})=>{const H="</"+S[0].slice(1);return S.input.indexOf(H,M)!==-1},a=ot,o={begin:"<>",end:"</>"},d=/<[A-Za-z0-9\\._:-]+\s*\/>/,i={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(S,M)=>{const H=S[0].length+S.index,B=S.input[H];if(B==="<"||B===","){M.ignoreMatch();return}B===">"&&(n(S,{after:H})||M.ignoreMatch());let P;const Y=S.input.substring(H);if(P=Y.match(/^\s*=/)){M.ignoreMatch();return}if((P=Y.match(/^\s+extends\s+/))&&P.index===0){M.ignoreMatch();return}}},l={$pattern:ot,keyword:oa,literal:sa,built_in:ca,"variable.language":ia},p="[0-9](_?[0-9])*",f=`\\.(${p})`,m="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",v={className:"number",variants:[{begin:`(\\b(${m})((${f})|\\.)?|(${f}))[eE][+-]?(${p})\\b`},{begin:`\\b(${m})\\b((${f})\\b|\\.)?|(${f})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},u={className:"subst",begin:"\\$\\{",end:"\\}",keywords:l,contains:[]},b={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"xml"}},_={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"css"}},c={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"graphql"}},g={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,u]},R={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},h=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,_,c,g,{match:/\$\d+/},v];u.contains=h.concat({begin:/\{/,end:/\}/,keywords:l,contains:["self"].concat(h)});const w=[].concat(R,u.contains),I=w.concat([{begin:/(\s*)\(/,end:/\)/,keywords:l,contains:["self"].concat(w)}]),x={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:l,contains:I},K={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},q={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...dt,...ut]}},ee={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[x],illegal:/%/},z={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function Z(S){return t.concat("(?!",S.join("|"),")")}const G={match:t.concat(/\b/,Z([...pt,"super","import"].map(S=>`${S}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},D={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},C={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},x]},$="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",L={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead($)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[x]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:l,exports:{PARAMS_CONTAINS:I,CLASS_REFERENCE:q},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),ee,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,_,c,g,R,{match:/\$\d+/},v,q,{scope:"attr",match:a+t.lookahead(":"),relevance:0},L,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[R,e.REGEXP_MODE,{className:"function",begin:$,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:l,contains:I}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:o.begin,end:o.end},{match:d},{begin:i.begin,"on:begin":i.isTrulyOpeningTag,end:i.end}],subLanguage:"xml",contains:[{begin:i.begin,end:i.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[x,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},D,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[x]},G,z,K,C,{match:/\$[(.]/}]}}const Ye={name:"javascript",register:la};function da(e){const t=e.regex,n={},a={begin:/\$\{/,end:/\}/,contains:["self",{begin:/:-/,contains:[n]}]};Object.assign(n,{className:"variable",variants:[{begin:t.concat(/\$[\w\d#@][\w\d_]*/,"(?![\\w\\d])(?![$])")},a]});const o={className:"subst",begin:/\$\(/,end:/\)/,contains:[e.BACKSLASH_ESCAPE]},d=e.inherit(e.COMMENT(),{match:[/(^|\s)/,/#.*$/],scope:{2:"comment"}}),i={begin:/<<-?\s*(?=\w+)/,starts:{contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,className:"string"})]}},l={className:"string",begin:/"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,n,o]};o.contains.push(l);const p={match:/\\"/},f={className:"string",begin:/'/,end:/'/},m={match:/\\'/},v={begin:/\$?\(\(/,end:/\)\)/,contains:[{begin:/\d+#[0-9a-f]+/,className:"number"},e.NUMBER_MODE,n]},u=["fish","bash","zsh","sh","csh","ksh","tcsh","dash","scsh"],b=e.SHEBANG({binary:`(${u.join("|")})`,relevance:10}),_={className:"function",begin:/\w[\w\d_]*\s*\(\s*\)\s*\{/,returnBegin:!0,contains:[e.inherit(e.TITLE_MODE,{begin:/\w[\w\d_]*/})],relevance:0},c=["if","then","else","elif","fi","time","for","while","until","in","do","done","case","esac","coproc","function","select"],g=["true","false"],O={match:/(\/[a-z._-]+)+/},R=["break","cd","continue","eval","exec","exit","export","getopts","hash","pwd","readonly","return","shift","test","times","trap","umask","unset"],h=["alias","bind","builtin","caller","command","declare","echo","enable","help","let","local","logout","mapfile","printf","read","readarray","source","sudo","type","typeset","ulimit","unalias"],w=["autoload","bg","bindkey","bye","cap","chdir","clone","comparguments","compcall","compctl","compdescribe","compfiles","compgroups","compquote","comptags","comptry","compvalues","dirs","disable","disown","echotc","echoti","emulate","fc","fg","float","functions","getcap","getln","history","integer","jobs","kill","limit","log","noglob","popd","print","pushd","pushln","rehash","sched","setcap","setopt","stat","suspend","ttyctl","unfunction","unhash","unlimit","unsetopt","vared","wait","whence","where","which","zcompile","zformat","zftp","zle","zmodload","zparseopts","zprof","zpty","zregexparse","zsocket","zstyle","ztcp"],I=["chcon","chgrp","chown","chmod","cp","dd","df","dir","dircolors","ln","ls","mkdir","mkfifo","mknod","mktemp","mv","realpath","rm","rmdir","shred","sync","touch","truncate","vdir","b2sum","base32","base64","cat","cksum","comm","csplit","cut","expand","fmt","fold","head","join","md5sum","nl","numfmt","od","paste","ptx","pr","sha1sum","sha224sum","sha256sum","sha384sum","sha512sum","shuf","sort","split","sum","tac","tail","tr","tsort","unexpand","uniq","wc","arch","basename","chroot","date","dirname","du","echo","env","expr","factor","groups","hostid","id","link","logname","nice","nohup","nproc","pathchk","pinky","printenv","printf","pwd","readlink","runcon","seq","sleep","stat","stdbuf","stty","tee","test","timeout","tty","uname","unlink","uptime","users","who","whoami","yes"];return{name:"Bash",aliases:["sh","zsh"],keywords:{$pattern:/\b[a-z][a-z0-9._-]+\b/,keyword:c,literal:g,built_in:[...R,...h,"set","shopt",...w,...I]},contains:[b,e.SHEBANG(),_,v,d,i,O,l,p,f,m,n]}}const Fe={name:"bash",register:da};function ua(e){const t=e.regex,n=/(?![A-Za-z0-9])(?![$])/,a=t.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,n),o=t.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,n),d=t.concat(/[A-Z]+/,n),i={scope:"variable",match:"\\$+"+a},l={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},p={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},f=e.inherit(e.APOS_STRING_MODE,{illegal:null}),m=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(p)}),v={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(p),"on:begin":(D,C)=>{C.data._beginMatch=D[1]||D[2]},"on:end":(D,C)=>{C.data._beginMatch!==D[1]&&C.ignoreMatch()}},u=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),b=`[ 	
]`,_={scope:"string",variants:[m,f,v,u]},c={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},g=["false","null","true"],O=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],R=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],w={keyword:O,literal:(D=>{const C=[];return D.forEach($=>{C.push($),$.toLowerCase()===$?C.push($.toUpperCase()):C.push($.toLowerCase())}),C})(g),built_in:R},I=D=>D.map(C=>C.replace(/\|\d+$/,"")),x={variants:[{match:[/new/,t.concat(b,"+"),t.concat("(?!",I(R).join("\\b|"),"\\b)"),o],scope:{1:"keyword",4:"title.class"}}]},K=t.concat(a,"\\b(?!\\()"),q={variants:[{match:[t.concat(/::/,t.lookahead(/(?!class\b)/)),K],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[o,t.concat(/::/,t.lookahead(/(?!class\b)/)),K],scope:{1:"title.class",3:"variable.constant"}},{match:[o,t.concat("::",t.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[o,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},ee={scope:"attr",match:t.concat(a,t.lookahead(":"),t.lookahead(/(?!::)/))},J={relevance:0,begin:/\(/,end:/\)/,keywords:w,contains:[ee,i,q,e.C_BLOCK_COMMENT_MODE,_,c,x]},z={relevance:0,match:[/\b/,t.concat("(?!fn\\b|function\\b|",I(O).join("\\b|"),"|",I(R).join("\\b|"),"\\b)"),a,t.concat(b,"*"),t.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[J]};J.contains.push(z);const Z=[ee,q,e.C_BLOCK_COMMENT_MODE,_,c,x],G={begin:t.concat(/#\[\s*\\?/,t.either(o,d)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:g,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:g,keyword:["new","array"]},contains:["self",...Z]},...Z,{scope:"meta",variants:[{match:o},{match:d}]}]};return{case_insensitive:!1,keywords:w,contains:[G,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},l,{scope:"variable.language",match:/\$this\b/},i,z,q,{match:[/const/,/\s/,a],scope:{1:"keyword",3:"variable.constant"}},x,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:w,contains:["self",G,i,q,e.C_BLOCK_COMMENT_MODE,_,c]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},_,c]}}const zn={name:"php",register:ua};function pa(e){const t=e.regex,n=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),a=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],l={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:a,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},p={className:"meta",begin:/^(>>>|\.\.\.) /},f={className:"subst",begin:/\{/,end:/\}/,keywords:l,illegal:/#/},m={begin:/\{\{/,relevance:0},v={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,p],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,p],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,p,m,f]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,p,m,f]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,m,f]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,m,f]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},u="[0-9](_?[0-9])*",b=`(\\b(${u}))?\\.(${u})|\\b(${u})\\.`,_=`\\b|${a.join("|")}`,c={className:"number",relevance:0,variants:[{begin:`(\\b(${u})|(${b}))[eE][+-]?(${u})[jJ]?(?=${_})`},{begin:`(${b})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${_})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${_})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${_})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${_})`},{begin:`\\b(${u})[jJ](?=${_})`}]},g={className:"comment",begin:t.lookahead(/# type:/),end:/$/,keywords:l,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},O={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:l,contains:["self",p,c,v,e.HASH_COMMENT_MODE]}]};return f.contains=[v,c,p],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:l,illegal:/(<\/|\?)|=>/,contains:[p,c,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},v,g,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[O]},{variants:[{match:[/\bclass/,/\s+/,n,/\s*/,/\(\s*/,n,/\s*\)/]},{match:[/\bclass/,/\s+/,n]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[c,O,v]}]}}const ma={name:"python",register:pa};function Hn(e){const t="go get go.tracewayapp.com";switch(e){case"gin":return`${t} && go get go.tracewayapp.com/tracewaygin`;case"chi":return`${t} && go get go.tracewayapp.com/tracewaychi`;case"fiber":return`${t} && go get go.tracewayapp.com/tracewayfiber`;case"fasthttp":return`${t} && go get go.tracewayapp.com/tracewayfasthttp`;case"stdlib":return`${t} && go get go.tracewayapp.com/tracewayhttp`;case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.0")';default:return t}}function qn(e,t,n){const a=t?`${t}@${n}/api/report`:`YOUR_TOKEN@${n}/api/report`;switch(e){case"gin":return`package main

import (
    "github.com/gin-gonic/gin"
    tracewaygin "go.tracewayapp.com/tracewaygin"
)

func main() {
    r := gin.Default()
    r.Use(tracewaygin.New("${a}"))
    r.Run(":8080")
}`;case"chi":return`package main

import (
    "net/http"

    "github.com/go-chi/chi/v5"
    tracewaychi "go.tracewayapp.com/tracewaychi"
)

func main() {
    r := chi.NewRouter()
    r.Use(tracewaychi.New("${a}"))

    r.Get("/api/users", getUsers)
    http.ListenAndServe(":8080", r)
}`;case"fiber":return`package main

import (
    "github.com/gofiber/fiber/v2"
    tracewayfiber "go.tracewayapp.com/tracewayfiber"
)

func main() {
    app := fiber.New()
    app.Use(tracewayfiber.New("${a}"))

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

    tracedHandler := tracewayfasthttp.New("${a}")(handler)
    fasthttp.ListenAndServe(":8080", tracedHandler)
}`;case"stdlib":return`package main

import (
    "net/http"

    tracewayhttp "go.tracewayapp.com/tracewayhttp"
)

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/api/users", getUsers)

    handler := tracewayhttp.New("${a}")(mux)
    http.ListenAndServe(":8080", handler)
}`;case"react":return`import { TracewayProvider } from "@tracewayapp/react";

function App() {
  return (
    <TracewayProvider connectionString="${a}">
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
      connectionString: "${a}",
    });
  }
<\/script>

<slot />`;case"vuejs":return`import { createApp } from "vue";
import { createTracewayPlugin } from "@tracewayapp/vue";
import App from "./App.vue";

const app = createApp(App);

app.use(createTracewayPlugin({
  connectionString: "${a}",
}));

app.mount("#app");`;case"nextjs":return`// app/traceway-provider.tsx
"use client";

import { TracewayProvider } from "@tracewayapp/react";

export function TracewayClientProvider({ children }: { children: React.ReactNode }) {
  return (
    <TracewayProvider connectionString="${a}">
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
            connectionString: "${a}",
        }),
    ],
})
export class AppModule {}`;case"express":return`import express from "express";
import { traceway } from "@tracewayapp/express";

const app = express();
app.use(traceway("${a}"));

app.get("/api/users", (req, res) => {
    res.json({ users: [] });
});

app.listen(8080);`;case"remix":return`import { withTraceway } from "@tracewayapp/remix";

export default withTraceway({
    connectionString: "${a}",
});`;case"jquery":return`import { init } from "@tracewayapp/jquery";

init("${a}");

// jQuery AJAX errors are captured automatically
// Distributed trace headers are injected into $.ajax() requests`;case"react-native":return`import { TracewayProvider } from "@tracewayapp/react-native";

export default function App() {
  return (
    <TracewayProvider connectionString="${a}">
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
// .env  — point the OTLP exporter at Traceway
//
// OTEL_SERVICE_NAME=my-laravel-app
// OTEL_TRACES_EXPORTER=otlp
// OTEL_METRICS_EXPORTER=otlp
// OTEL_LOGS_EXPORTER=otlp
// OTEL_EXPORTER_OTLP_PROTOCOL=http/json
// OTEL_EXPORTER_OTLP_ENDPOINT=${n}/api/otel
// OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer ${t||"YOUR_TOKEN"}"
//
// Optional: send Laravel logs to Traceway via the auto-injected 'otlp' channel
// LOG_CHANNEL=otlp

// That's it — keepsuit/laravel-opentelemetry's service provider auto-registers
// TraceRequestMiddleware as a global middleware, so every HTTP request, DB query,
// queued job, Redis call, cache op, view render and outbound Http:: call is
// traced automatically. Open config/opentelemetry.php to tune which
// instrumentations are enabled.`;case"django":return`# .env  — point the OTLP exporter at Traceway
#
# OTEL_SERVICE_NAME=my-django-app
# OTEL_TRACES_EXPORTER=otlp
# OTEL_METRICS_EXPORTER=otlp
# OTEL_LOGS_EXPORTER=otlp
# OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
# OTEL_EXPORTER_OTLP_ENDPOINT=${n}/api/otel
# OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20${t||"YOUR_TOKEN"}
# OTEL_PYTHON_LOGGING_AUTO_INSTRUMENTATION_ENABLED=true

# Then launch Django through the OTel agent — no code changes needed:
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
    connectionString: '${a}',
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
            connectionString = "${a}",
            options = TracewayOptions(version = "1.0.0"),
        )
    }
}`;default:return`package main

import (
    "go.tracewayapp.com"
)

func main() {
    traceway.Init(
        "${a}",
        traceway.WithVersion("1.0.0"),
        traceway.WithServerName("my-server"),
    )
}`}}function Wn(e){return e==="symfony"?`<?php
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
throw RuntimeException("Test error from Traceway integration")`:e&&Xe(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:`r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`}function Zn(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

TracewayClient.instance?.captureException(
  Exception('Test error'),
  StackTrace.current,
);`;if(e==="android")return`import com.tracewayapp.traceway.Traceway

try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e)
}`;if(e&&Xe(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

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
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${ga(e)}";

captureException(new Error("Test error"));`}return`r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`}function ga(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function Yn(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android"}[e]||e}function Xn(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||Xe(e)?"javascript":"go"}const Re=[{id:"collector",label:"Collector",frameworks:[]},{id:"nodejs",label:"Node.js",frameworks:[{id:"express",label:"Express"},{id:"nestjs",label:"NestJS"},{id:"fastify",label:"Fastify"},{id:"nextjs",label:"Next.js"},{id:"koa",label:"Koa"},{id:"other",label:"Other"}]},{id:"go",label:"Go",frameworks:[{id:"gin",label:"Gin"},{id:"echo",label:"Echo"},{id:"chi",label:"Chi"},{id:"fiber",label:"Fiber"},{id:"mux",label:"gorilla/mux"},{id:"nethttp",label:"net/http"}]},{id:"python",label:"Python",frameworks:[{id:"django",label:"Django"},{id:"flask",label:"Flask"},{id:"fastapi",label:"FastAPI"},{id:"other",label:"Other"}]},{id:"java",label:"Java",frameworks:[{id:"agent",label:"Any framework"},{id:"spring",label:"Spring Boot"}]},{id:"dotnet",label:".NET",frameworks:[]},{id:"php",label:"PHP",frameworks:[{id:"symfony",label:"Symfony"},{id:"laravel",label:"Laravel"},{id:"slim",label:"Slim"},{id:"other",label:"Other"}]},{id:"ruby",label:"Ruby",frameworks:[{id:"rails",label:"Rails"},{id:"other",label:"Other"}]},{id:"other",label:"Other",frameworks:[]}];function mt(e,t,n=[]){return["OTEL_SERVICE_NAME=my-service",`OTEL_EXPORTER_OTLP_ENDPOINT=${e}/api/otel`,`OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer ${t}`,...n].join(`
`)}function pe(e,t,n=[],a=""){return{title:"Configure the Exporter",description:`Set these environment variables in your shell, .env file, or deployment config. The SDK appends /v1/traces and /v1/metrics to the endpoint automatically.${a?" "+a:""}`,code:mt(e,t,n),codeLanguage:"bash"}}function _a(e,t){return`exporters:
  otlphttp:
    endpoint: "${e}/api/otel"
    headers:
      Authorization: "Bearer ${t}"

service:
  pipelines:
    traces:
      exporters: [otlphttp]
    metrics:
      exporters: [otlphttp]`}const ba=`import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func initTracer(ctx context.Context) *sdktrace.TracerProvider {
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		log.Fatal(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp
}`,st={gin:{lib:"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin",snippet:`import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

r := gin.Default()
r.Use(otelgin.Middleware("my-service"))`},echo:{lib:"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho",snippet:`import "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"

e := echo.New()
e.Use(otelecho.Middleware("my-service"))`},chi:{lib:"github.com/riandyrn/otelchi",snippet:`import "github.com/riandyrn/otelchi"

r := chi.NewRouter()
r.Use(otelchi.Middleware("my-service", otelchi.WithChiRoutes(r)))`,note:"WithChiRoutes lets the middleware resolve the route pattern so endpoints group correctly."},fiber:{lib:"github.com/gofiber/contrib/v3/otel",snippet:`import fiberotel "github.com/gofiber/contrib/v3/otel"

app := fiber.New()
app.Use(fiberotel.Middleware())`,note:"For Fiber v2 use github.com/gofiber/contrib/otelfiber/v2 and otelfiber.Middleware() instead."},mux:{lib:"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux",snippet:`import "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"

r := mux.NewRouter()
r.Use(otelmux.Middleware("my-service"))`},nethttp:{lib:"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",snippet:`import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

mux := http.NewServeMux()
mux.Handle("GET /users/{id}", otelhttp.NewHandler(http.HandlerFunc(getUser), "GET /users/{id}"))
http.ListenAndServe(":8080", mux)`,note:"Wrap each route individually with Go 1.22+ method patterns so the route is set on spans and endpoints group by pattern instead of raw URL."}},fa="npm install @opentelemetry/api @opentelemetry/auto-instrumentations-node";function Ze(e,t,n,a,o){return[{title:"Install the SDK",description:a,code:fa,codeLanguage:"bash"},pe(e,t),{title:"Run with Instrumentation",description:o,code:`node --require @opentelemetry/auto-instrumentations-node/register ${n}`,codeLanguage:"bash"}]}function va(e,t,n,a){switch(e){case"collector":return[{title:"Add the Traceway Exporter",description:"Merge this into your OpenTelemetry Collector configuration. Any pipeline that lists the otlphttp exporter will be forwarded to Traceway.",code:_a(n,a),codeLanguage:"yaml"},{title:"Restart the Collector",description:"Restart the Collector to apply the configuration. Traces and metrics flowing through its pipelines will appear in Traceway."}];case"nodejs":return t==="fastify"?[{title:"Install the SDK",description:"Fastify is instrumented by the @fastify/otel package maintained by the Fastify team.",code:"npm install @opentelemetry/api @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @fastify/otel",codeLanguage:"bash"},{title:"Create instrumentation.js",description:"Add this file at the project root.",code:`const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { FastifyOtelInstrumentation } = require('@fastify/otel');

new NodeSDK({
  instrumentations: [
    getNodeAutoInstrumentations(),
    new FastifyOtelInstrumentation({ registerOnInitialization: true }),
  ],
}).start();`,codeLanguage:"javascript"},pe(n,a),{title:"Run with Instrumentation",code:"node --require ./instrumentation.js app.js",codeLanguage:"bash"}]:t==="nextjs"?[{title:"Install the SDK",code:"npm install @vercel/otel",codeLanguage:"bash"},{title:"Create instrumentation.ts",description:"Add this file at the project root (next to package.json). Next.js calls register() automatically on startup.",code:`import { registerOTel } from '@vercel/otel'

export function register() {
  registerOTel({ serviceName: 'my-service' })
}`,codeLanguage:"typescript"},pe(n,a,[],"Start your app normally with next start; no extra flags are needed.")]:t==="nestjs"?Ze(n,a,"dist/main.js","Auto-instrumentation captures NestJS routes, status codes, and errors through the default Express adapter with no code changes. If you use the Fastify adapter, follow the Fastify setup instead.","Routes group by pattern automatically."):t==="koa"?Ze(n,a,"app.js","Auto-instrumentation captures Koa requests, status codes, and errors with no code changes.","Route patterns are captured when routing with @koa/router."):Ze(n,a,"app.js","Auto-instrumentation captures routes, status codes, and errors with no code changes.","For ESM apps, add --experimental-loader=@opentelemetry/instrumentation/hook.mjs and use --import instead of --require.");case"go":{const o=st[t]??st.gin;return[{title:"Install the SDK",code:`go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp ${o.lib}`,codeLanguage:"bash"},{title:"Initialize the SDK",description:"Call initTracer at startup and defer tp.Shutdown(ctx) before exit. The exporter reads the environment variables from the next step.",code:ba,codeLanguage:"go"},{title:"Add the Middleware",description:o.note,code:o.snippet,codeLanguage:"go"},pe(n,a)]}case"python":{const o={django:{cmd:"opentelemetry-instrument python manage.py runserver --noreload",note:"The --noreload flag is required with runserver; the autoreloader breaks instrumentation. It is not needed under gunicorn or other production servers."},flask:{cmd:"opentelemetry-instrument flask run"},fastapi:{cmd:"opentelemetry-instrument uvicorn main:app",note:"Avoid --reload and --workers with zero-code instrumentation; for multi-worker production use gunicorn with uvicorn workers."},other:{cmd:"opentelemetry-instrument python app.py"}},d=o[t]??o.other;return[{title:"Install the SDK",description:"opentelemetry-bootstrap detects your installed packages and adds the matching instrumentation.",code:`pip install opentelemetry-distro opentelemetry-exporter-otlp-proto-http
opentelemetry-bootstrap -a install`,codeLanguage:"bash"},pe(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the Python SDK defaults to gRPC."),{title:"Run with Instrumentation",description:d.note,code:d.cmd,codeLanguage:"bash"}]}case"java":return t==="spring"?[{title:"Add the Starter",description:"Add the OpenTelemetry Spring Boot starter to your Gradle build (a Maven dependency works the same way).",code:`implementation(platform("io.opentelemetry.instrumentation:opentelemetry-instrumentation-bom:2.28.1"))
implementation("io.opentelemetry.instrumentation:opentelemetry-spring-boot-starter")`,codeLanguage:"gradle"},pe(n,a,[],"Start your app normally; the starter reads these variables and reports routes, status codes, and exceptions.")]:[{title:"Download the Java Agent",description:"The agent instruments Spring, JAX-RS, and most Java frameworks with zero code changes.",code:"curl -L -O https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar",codeLanguage:"bash"},pe(n,a),{title:"Run with the Agent",code:"java -javaagent:./opentelemetry-javaagent.jar -jar myapp.jar",codeLanguage:"bash"}];case"dotnet":return[{title:"Install the Packages",code:`dotnet add package OpenTelemetry.Extensions.Hosting
dotnet add package OpenTelemetry.Instrumentation.AspNetCore
dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol`,codeLanguage:"bash"},{title:"Add to Program.cs",description:"Keep AddOtlpExporter() empty so the exporter is driven entirely by the environment variables in the next step.",code:`builder.Services.AddOpenTelemetry()
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddOtlpExporter());`,codeLanguage:"csharp"},pe(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the .NET exporter defaults to gRPC.")];case"php":{const d={symfony:" open-telemetry/opentelemetry-auto-symfony",laravel:" open-telemetry/opentelemetry-auto-laravel",slim:" open-telemetry/opentelemetry-auto-slim",other:""}[t]??"";return[{title:"Install the SDK",description:"Auto-instrumentation needs the opentelemetry PECL extension; enable it with extension=opentelemetry in php.ini."+(t==="other"?" Find auto-instrumentation packages for your framework in the OpenTelemetry registry.":""),code:`pecl install opentelemetry
composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter${d}`,codeLanguage:"bash",link:t==="other"?{label:"Browse PHP instrumentation packages",href:"https://opentelemetry.io/ecosystem/registry/?language=php&component=instrumentation"}:void 0},pe(n,a,["OTEL_PHP_AUTOLOAD_ENABLED=true"],"These must be real process environment variables; the extension does not read framework .env files. Use env[...] in php-fpm pool config or SetEnv in Apache.")]}case"ruby":return t==="rails"?[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-rails",codeLanguage:"bash"},{title:"Create the Initializer",description:"Add config/initializers/opentelemetry.rb.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/rails'

OpenTelemetry::SDK.configure do |c|
  c.use 'OpenTelemetry::Instrumentation::Rails'
end`,codeLanguage:"ruby"},pe(n,a)]:[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-all",codeLanguage:"bash"},{title:"Configure the SDK",description:"Run this once at startup, before your app starts handling requests.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure do |c|
  c.use_all
end`,codeLanguage:"ruby"},pe(n,a)];default:return[{title:"Configure any OpenTelemetry SDK",description:"Any language with an OTLP/HTTP exporter works. Set these environment variables; the protocol variable matters for SDKs that default to gRPC. Make sure http.route is set on root server spans so endpoints group by route pattern, and use SpanKind CONSUMER for background jobs.",code:mt(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"]),codeLanguage:"bash",link:{label:"View all supported languages",href:"https://opentelemetry.io/docs/languages/"}}]}}const gt="traceway_setup_mode",_t="traceway_otel_language",bt="traceway_otel_framework";function Vn(){try{const e=localStorage.getItem(gt);if(e==="ai"||e==="manual")return e}catch{}return"ai"}function Ea(e){try{localStorage.setItem(gt,e)}catch{}}function ya(){try{const e=localStorage.getItem(_t);if(e&&Re.some(t=>t.id===e))return e}catch{}return Re[0].id}function ha(e){try{localStorage.setItem(_t,e)}catch{}}function Ta(){try{return localStorage.getItem(bt)}catch{}return null}function wa(e){try{localStorage.setItem(bt,e)}catch{}}var Sa=N("<!> <!>",1);function Qn(e,t){Se(t,!0);const n="-mb-px rounded-none border-b-2 border-transparent bg-transparent px-0 pb-2.5 pt-0 text-sm font-medium text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none";function a(i){(i==="ai"||i==="manual")&&(Ea(i),t.onModeChange(i))}var o=F(),d=E(o);V(d,()=>Ue,(i,l)=>{l(i,{get value(){return t.mode},onValueChange:a,children:(p,f)=>{var m=F(),v=E(m);V(v,()=>Be,(u,b)=>{b(u,{class:"h-auto w-full justify-start gap-4 rounded-none border-b bg-transparent p-0",children:(_,c)=>{var g=Sa(),O=E(g);V(O,()=>Me,(h,w)=>{w(h,{value:"ai",class:n,children:(I,x)=>{U();var K=ge("AI");s(I,K)},$$slots:{default:!0}})});var R=A(O,2);V(R,()=>Me,(h,w)=>{w(h,{value:"manual",class:n,children:(I,x)=>{U();var K=ge("Manual");s(I,K)},$$slots:{default:!0}})}),s(_,g)},$$slots:{default:!0}})}),s(p,m)},$$slots:{default:!0}})}),s(e,o),Ae()}const Aa="npx skills add tracewayapp/traceway";function Oa(e,t,n=null){const a=[{text:"/traceway-setup with token ",bold:!1},{text:t,bold:!0},{text:" and url ",bold:!1},{text:e,bold:!0}];return n&&a.push({text:" and source map upload token ",bold:!1},{text:n,bold:!0}),a}var Na=N("<!> Copied!",1),Ra=N("<!> Copy",1),xa=N('<div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div>');function Ve(e,t){Se(t,!0);let n=Ut(t,"wrap",3,!1),a=ue(!1);async function o(){await navigator.clipboard.writeText(t.code),k(a,!0),setTimeout(()=>k(a,!1),2e3)}var d=xa(),i=T(d),l=T(i);me(l,{variant:"outline",size:"sm",onclick:o,children:(m,v)=>{var u=F(),b=E(u);{var _=g=>{var O=Na(),R=E(O);Te(R,{class:"mr-2 h-4 w-4 text-green-500"}),U(),s(g,O)},c=g=>{var O=Ra(),R=E(O);we(R,{class:"mr-2 h-4 w-4"}),U(),s(g,O)};Q(b,g=>{r(a)?g(_):g(c,!1)})}s(m,u)},$$slots:{default:!0}}),y(i);var p=A(i,2),f=T(p);ke(f,{get language(){return t.language},get code(){return t.code}}),y(p),y(d),le(()=>Pe(p,1,`overflow-x-auto rounded-lg text-sm ${n()?"wrap-code":""} ${De.isDark?"dark-code":"light-code"}`)),s(e,d),Ae()}var Ca=N("<!> Copied!",1),Ia=N("<!> Copy",1),$a=N('<span class="break-all text-muted-foreground"> </span>'),Ma=N('<div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <code class="block rounded-lg bg-muted py-3 pr-24 pl-4 font-mono text-sm break-words whitespace-pre-wrap text-foreground"></code></div>');function La(e,t){Se(t,!0);const n=oe(()=>t.parts.map(f=>f.text).join(""));let a=ue(!1);async function o(){await navigator.clipboard.writeText(r(n)),k(a,!0),setTimeout(()=>k(a,!1),2e3)}var d=Ma(),i=T(d),l=T(i);me(l,{variant:"outline",size:"sm",onclick:o,children:(f,m)=>{var v=F(),u=E(v);{var b=c=>{var g=Ca(),O=E(g);Te(O,{class:"mr-2 h-4 w-4 text-green-500"}),U(),s(c,g)},_=c=>{var g=Ia(),O=E(g);we(O,{class:"mr-2 h-4 w-4"}),U(),s(c,g)};Q(u,c=>{r(a)?c(b):c(_,!1)})}s(f,v)},$$slots:{default:!0}}),y(i);var p=A(i,2);$e(p,21,()=>t.parts,Kt,(f,m)=>{var v=F(),u=E(v);{var b=c=>{var g=$a(),O=T(g,!0);y(g),le(()=>re(O,r(m).text)),s(c,g)},_=c=>{var g=ge();le(()=>re(g,r(m).text)),s(c,g)};Q(u,c=>{r(m).bold?c(b):c(_,!1)})}s(f,v)}),y(p),y(d),s(e,d),Ae()}var Pa=N(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function Jn(e,t){Se(t,!0);const n=oe(()=>xe.currentProject?.sourceMapToken??null),a=oe(()=>Oa(t.backendUrl,t.token,r(n)));var o=Pa(),d=E(o),i=A(T(d),2),l=T(i);Ve(l,{get code(){return Aa},get language(){return Fe}}),y(i),y(d);var p=A(d,2),f=T(p),m=A(T(f),2),v=T(m);y(m),y(f);var u=A(f,2),b=T(u);La(b,{get parts(){return r(a)}}),y(u),y(p),le(()=>re(v,`Paste this prompt into your agent. Your instance URL and project token are already filled
			in${r(n)?", along with your source map upload token":""}.`)),s(e,o),Ae()}const Ke="[A-Za-z$_][0-9A-Za-z$_]*",ft=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],vt=["true","false","null","undefined","NaN","Infinity"],Et=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],yt=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],ht=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],Tt=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],wt=[].concat(ht,Et,yt);function ka(e){const t=e.regex,n=(S,{after:M})=>{const H="</"+S[0].slice(1);return S.input.indexOf(H,M)!==-1},a=Ke,o={begin:"<>",end:"</>"},d=/<[A-Za-z0-9\\._:-]+\s*\/>/,i={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(S,M)=>{const H=S[0].length+S.index,B=S.input[H];if(B==="<"||B===","){M.ignoreMatch();return}B===">"&&(n(S,{after:H})||M.ignoreMatch());let P;const Y=S.input.substring(H);if(P=Y.match(/^\s*=/)){M.ignoreMatch();return}if((P=Y.match(/^\s+extends\s+/))&&P.index===0){M.ignoreMatch();return}}},l={$pattern:Ke,keyword:ft,literal:vt,built_in:wt,"variable.language":Tt},p="[0-9](_?[0-9])*",f=`\\.(${p})`,m="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",v={className:"number",variants:[{begin:`(\\b(${m})((${f})|\\.)?|(${f}))[eE][+-]?(${p})\\b`},{begin:`\\b(${m})\\b((${f})\\b|\\.)?|(${f})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},u={className:"subst",begin:"\\$\\{",end:"\\}",keywords:l,contains:[]},b={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"xml"}},_={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"css"}},c={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,u],subLanguage:"graphql"}},g={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,u]},R={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},h=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,_,c,g,{match:/\$\d+/},v];u.contains=h.concat({begin:/\{/,end:/\}/,keywords:l,contains:["self"].concat(h)});const w=[].concat(R,u.contains),I=w.concat([{begin:/(\s*)\(/,end:/\)/,keywords:l,contains:["self"].concat(w)}]),x={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:l,contains:I},K={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},q={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...Et,...yt]}},ee={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},J={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[x],illegal:/%/},z={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function Z(S){return t.concat("(?!",S.join("|"),")")}const G={match:t.concat(/\b/,Z([...ht,"super","import"].map(S=>`${S}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},D={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},C={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},x]},$="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",L={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead($)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[x]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:l,exports:{PARAMS_CONTAINS:I,CLASS_REFERENCE:q},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),ee,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,_,c,g,R,{match:/\$\d+/},v,q,{scope:"attr",match:a+t.lookahead(":"),relevance:0},L,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[R,e.REGEXP_MODE,{className:"function",begin:$,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:l,contains:I}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:o.begin,end:o.end},{match:d},{begin:i.begin,"on:begin":i.isTrulyOpeningTag,end:i.end}],subLanguage:"xml",contains:[{begin:i.begin,end:i.end,skip:!0,contains:["self"]}]}]},J,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[x,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},D,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[x]},G,z,K,C,{match:/\$[(.]/}]}}function Da(e){const t=e.regex,n=ka(e),a=Ke,o=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],d={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},i={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:o},contains:[n.exports.CLASS_REFERENCE]},l={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},p=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],f={$pattern:Ke,keyword:ft.concat(p),literal:vt,built_in:wt.concat(o),"variable.language":Tt},m={className:"meta",begin:"@"+a},v=(c,g,O)=>{const R=c.contains.findIndex(h=>h.label===g);if(R===-1)throw new Error("can not find mode to replace");c.contains.splice(R,1,O)};Object.assign(n.keywords,f),n.exports.PARAMS_CONTAINS.push(m);const u=n.contains.find(c=>c.scope==="attr"),b=Object.assign({},u,{match:t.concat(a,t.lookahead(/\s*\?:/))});n.exports.PARAMS_CONTAINS.push([n.exports.CLASS_REFERENCE,u,b]),n.contains=n.contains.concat([m,d,i,b]),v(n,"shebang",e.SHEBANG()),v(n,"use_strict",l);const _=n.contains.find(c=>c.label==="func.def");return _.relevance=0,Object.assign(n,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),n}const St={name:"typescript",register:Da};function Ba(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const Ua={name:"gradle",register:Ba};function Fa(e){const t=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],n=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],a=["default","false","null","true"],o=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],d=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],i={keyword:o.concat(d),built_in:t,literal:a},l=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),p={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},f={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},m={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},v=e.inherit(m,{illegal:/\n/}),u={className:"subst",begin:/\{/,end:/\}/,keywords:i},b=e.inherit(u,{illegal:/\n/}),_={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,b]},c={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},u]},g=e.inherit(c,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},b]});u.contains=[c,_,m,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,p,e.C_BLOCK_COMMENT_MODE],b.contains=[g,_,v,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,p,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const O={variants:[f,c,_,m,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},R={begin:"<",end:">",contains:[{beginKeywords:"in out"},l]},h=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",w={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:i,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},O,p,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},l,R,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[l,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[l,R,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+h+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:i,contains:[{beginKeywords:n.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,R],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:i,relevance:0,contains:[O,p,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},w]}}const Ka={name:"csharp",register:Fa};function Ga(e){const t=e.regex,n="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",a=t.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),o=t.concat(a,/(::\w+)*/),i={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},l={className:"doctag",begin:"@[A-Za-z]+"},p={begin:"#<",end:">"},f=[e.COMMENT("#","$",{contains:[l]}),e.COMMENT("^=begin","^=end",{contains:[l],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],m={className:"subst",begin:/#\{/,end:/\}/,keywords:i},v={className:"string",contains:[e.BACKSLASH_ESCAPE,m],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:t.concat(/<<[-~]?'?/,t.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,m]})]}]},u="[1-9](_?[0-9])*|0",b="[0-9](_?[0-9])*",_={className:"number",relevance:0,variants:[{begin:`\\b(${u})(\\.(${b}))?([eE][+-]?(${b})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},c={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:i}]},x=[v,{variants:[{match:[/class\s+/,o,/\s+<\s+/,o]},{match:[/\b(class|module)\s+/,o]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:i},{match:[/(include|extend)\s+/,o],scope:{2:"title.class"},keywords:i},{relevance:0,match:[o,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:a,scope:"title.class"},{match:[/def/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[c]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[v,{begin:n}],relevance:0},_,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:i},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,m],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(p,f),relevance:0}].concat(p,f);m.contains=x,c.contains=x;const J=[{begin:/^\s*=>/,starts:{end:"$",contains:x}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:i,contains:x}}];return f.unshift(p),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:i,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(J).concat(f).concat(x)}}const za={name:"ruby",register:Ga};function Ha(e){const t="true false yes no null",n="[\\w#;/?:@&=+$,.~*'()[\\]]+",a={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},o={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},d={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},i={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,o]},l=e.inherit(i,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),u={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},b={end:",",endsWithParent:!0,excludeEnd:!0,keywords:t,relevance:0},_={begin:/\{/,end:/\}/,contains:[b],illegal:"\\n",relevance:0},c={begin:"\\[",end:"\\]",contains:[b],illegal:"\\n",relevance:0},g=[a,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+n},{className:"type",begin:"!<"+n+">"},{className:"type",begin:"!"+n},{className:"type",begin:"!!"+n},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:t,keywords:{literal:t}},u,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},_,c,d,i],O=[...g];return O.pop(),O.push(l),b.contains=O,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:g}}const At={name:"yaml",register:Ha};var qa=N("<!> Regenerate",1),Wa=N("<!> Copied!",1),Za=N("<!> Copy",1),Ya=N("<!> Copied!",1),Xa=N("<!> Copy",1),Va=N('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div>',1),Qa=N("<!> Copied!",1),Ja=N("<!> Copy",1),ja=N('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!> <!></div></div> <!> <div><p class="mb-2 text-sm font-medium"> </p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div></div>'),en=N(`<p class="text-sm text-muted-foreground">An upload token is required to upload source maps. Ask an organization admin to generate one
		from the Connection page.</p>`),tn=N("<!> Generating...",1),an=N("<!> Generate Upload Token",1),nn=N('<div class="flex items-center justify-between gap-4"><p class="text-sm text-muted-foreground">Generate an upload token to start uploading source maps as part of your build process.</p> <!></div>'),rn=N("<!> <!>",1),on=N("<!> <!>",1),sn=N(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1),cn=N("<!> <!>",1);function ln(e,t){Se(t,!0);const n={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:St,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:Ye,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:Ye,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let a=ue("vite"),o=ue(!1),d=ue(!1),i=ue(!1),l=ue(!1),p=ue(!1);const f="npm install -D @tracewayapp/bundler-plugin",m=oe(()=>xe.currentProject),v=oe(()=>r(m)?.sourceMapToken??null),u=oe(()=>ct.getRoleForOrganization(r(m)?.organizationId??0)==="readonly"),b=oe(()=>r(m)?.framework!=="react-native"),_=oe(()=>r(m)&&r(v)?`npx @tracewayapp/sourcemap-upload \\
  --url ${r(m).backendUrl} \\
  --token ${r(v)} \\
  --directory ${r(b)?n[r(a)].directory:"dist"}`:"");let c=ue(!1);async function g(){k(o,!0);try{await xe.generateSourceMapToken()}finally{k(o,!1)}}async function O(){k(o,!0);try{await xe.generateSourceMapToken(),k(c,!1),kt.success("Successfully regenerated the Upload Token",{position:"top-center"})}finally{k(o,!1)}}async function R(){r(v)&&(await navigator.clipboard.writeText(r(v)),k(d,!0),setTimeout(()=>k(d,!1),2e3))}async function h(){await navigator.clipboard.writeText(f),k(i,!0),setTimeout(()=>k(i,!1),2e3)}async function w(){await navigator.clipboard.writeText(n[r(a)].code),k(l,!0),setTimeout(()=>k(l,!1),2e3)}async function I(){await navigator.clipboard.writeText(r(_)),k(p,!0),setTimeout(()=>k(p,!1),2e3)}var x=cn(),K=E(x);{var q=z=>{var Z=ja(),G=T(Z),D=A(T(G),2),C=T(D),$=T(C,!0);y(C);var L=A(C,2);me(L,{variant:"outline",size:"sm",onclick:R,children:(ae,fe)=>{var X=F(),ce=E(X);{var te=W=>{Te(W,{class:"h-4 w-4 text-green-500"})},ye=W=>{we(W,{class:"h-4 w-4"})};Q(ce,W=>{r(d)?W(te):W(ye,!1)})}s(ae,X)},$$slots:{default:!0}});var S=A(L,2);me(S,{variant:"destructiveOutline",size:"sm",onclick:()=>k(c,!0),children:(ae,fe)=>{var X=qa(),ce=E(X);aa(ce,{class:"mr-2 h-4 w-4"}),U(),s(ae,X)},$$slots:{default:!0}}),y(D),y(G);var M=A(G,2);{var H=ae=>{var fe=Va(),X=E(fe),ce=A(T(X),2),te=T(ce),ye=T(te);me(ye,{variant:"outline",size:"sm",onclick:h,children:(Ce,qe)=>{var _e=F(),Le=E(_e);{var Ne=ne=>{var de=Wa(),ve=E(de);Te(ve,{class:"mr-2 h-4 w-4 text-green-500"}),U(),s(ne,de)},Ie=ne=>{var de=Za(),ve=E(de);we(ve,{class:"mr-2 h-4 w-4"}),U(),s(ne,de)};Q(Le,ne=>{r(i)?ne(Ne):ne(Ie,!1)})}s(Ce,_e)},$$slots:{default:!0}}),y(te);var W=A(te,2),he=T(W);ke(he,{get language(){return Fe},code:f}),y(W),y(ce),y(X);var Oe=A(X,2),Qe=A(T(Oe),2);V(Qe,()=>Ue,(Ce,qe)=>{qe(Ce,{get value(){return r(a)},onValueChange:_e=>{_e&&k(a,_e,!0)},children:(_e,Le)=>{var Ne=F(),Ie=E(Ne);V(Ie,()=>Be,(ne,de)=>{de(ne,{class:"mb-2",children:(ve,Tn)=>{var je=F(),xt=E(je);$e(xt,17,()=>Object.entries(n),([We,et])=>We,(We,et)=>{var tt=oe(()=>Ft(r(et),2));let Ct=()=>r(tt)[0],It=()=>r(tt)[1];var at=F(),$t=E(at);V($t,()=>Me,(Mt,Lt)=>{Lt(Mt,{get value(){return Ct()},children:(Pt,wn)=>{U();var nt=ge();le(()=>re(nt,It().label)),s(Pt,nt)},$$slots:{default:!0}})}),s(We,at)}),s(ve,je)},$$slots:{default:!0}})}),s(_e,Ne)},$$slots:{default:!0}})});var Ge=A(Qe,2),Ot=T(Ge,!0);y(Ge);var Je=A(Ge,2),ze=T(Je),Nt=T(ze);me(Nt,{variant:"outline",size:"sm",onclick:w,children:(Ce,qe)=>{var _e=F(),Le=E(_e);{var Ne=ne=>{var de=Ya(),ve=E(de);Te(ve,{class:"mr-2 h-4 w-4 text-green-500"}),U(),s(ne,de)},Ie=ne=>{var de=Xa(),ve=E(de);we(ve,{class:"mr-2 h-4 w-4"}),U(),s(ne,de)};Q(Le,ne=>{r(l)?ne(Ne):ne(Ie,!1)})}s(Ce,_e)},$$slots:{default:!0}}),y(ze);var He=A(ze,2),Rt=T(He);ke(Rt,{get language(){return n[r(a)].language},get code(){return n[r(a)].code}}),y(He),y(Je),y(Oe),le(()=>{Pe(W,1,`overflow-x-auto rounded-lg text-sm ${De.isDark?"dark-code":"light-code"}`),re(Ot,n[r(a)].file),Pe(He,1,`overflow-x-auto rounded-lg text-sm ${De.isDark?"dark-code":"light-code"}`)}),s(ae,fe)};Q(M,ae=>{r(b)&&ae(H)})}var B=A(M,2),P=T(B),Y=T(P,!0);y(P);var se=A(P,2),j=T(se),ie=T(j);me(ie,{variant:"outline",size:"sm",onclick:I,children:(ae,fe)=>{var X=F(),ce=E(X);{var te=W=>{var he=Qa(),Oe=E(he);Te(Oe,{class:"mr-2 h-4 w-4 text-green-500"}),U(),s(W,he)},ye=W=>{var he=Ja(),Oe=E(he);we(Oe,{class:"mr-2 h-4 w-4"}),U(),s(W,he)};Q(ce,W=>{r(p)?W(te):W(ye,!1)})}s(ae,X)},$$slots:{default:!0}}),y(j);var be=A(j,2),Ee=T(be);ke(Ee,{get language(){return Fe},get code(){return r(_)}}),y(be),y(se),y(B),y(Z),le(()=>{re($,r(v)),re(Y,r(b)?"Step 3: Upload after your production build":"Usage"),Pe(be,1,`overflow-x-auto rounded-lg text-sm ${De.isDark?"dark-code":"light-code"}`)}),s(z,Z)},ee=z=>{var Z=F(),G=E(Z);{var D=$=>{var L=en();s($,L)},C=$=>{var L=nn(),S=A(T(L),2);me(S,{variant:"outline",size:"sm",onclick:g,get disabled(){return r(o)},children:(M,H)=>{var B=F(),P=E(B);{var Y=j=>{var ie=tn(),be=E(ie);Xt(be,{class:"mr-2 h-4 w-4"}),U(),s(j,ie)},se=j=>{var ie=an(),be=E(ie);lt(be,{class:"mr-2 h-4 w-4"}),U(),s(j,ie)};Q(P,j=>{r(o)?j(Y):j(se,!1)})}s(M,B)},$$slots:{default:!0}}),y(L),s($,L)};Q(G,$=>{r(u)?$(D):$(C,!1)},!0)}s(z,Z)};Q(K,z=>{r(v)?z(q):z(ee,!1)})}var J=A(K,2);V(J,()=>ta,(z,Z)=>{Z(z,{get open(){return r(c)},set open(G){k(c,G,!0)},children:(G,D)=>{var C=F(),$=E(C);V($,()=>Vt,(L,S)=>{S(L,{interactOutsideBehavior:"close",children:(M,H)=>{var B=sn(),P=E(B);V(P,()=>Qt,(se,j)=>{j(se,{children:(ie,be)=>{var Ee=rn(),ae=E(Ee);V(ae,()=>Jt,(X,ce)=>{ce(X,{children:(te,ye)=>{U();var W=ge("Regenerate Upload Token");s(te,W)},$$slots:{default:!0}})});var fe=A(ae,2);V(fe,()=>jt,(X,ce)=>{ce(X,{children:(te,ye)=>{U();var W=ge(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);s(te,W)},$$slots:{default:!0}})}),s(ie,Ee)},$$slots:{default:!0}})});var Y=A(P,4);V(Y,()=>ea,(se,j)=>{j(se,{class:"sm:justify-between",children:(ie,be)=>{var Ee=on(),ae=E(Ee);me(ae,{variant:"outline",onclick:()=>k(c,!1),get disabled(){return r(o)},children:(X,ce)=>{U();var te=ge("Cancel");s(X,te)},$$slots:{default:!0}});var fe=A(ae,2);me(fe,{variant:"destructive",onclick:O,get disabled(){return r(o)},children:(X,ce)=>{U();var te=ge();le(()=>re(te,r(o)?"Regenerating...":"Regenerate Token")),s(X,te)},$$slots:{default:!0}}),s(ie,Ee)},$$slots:{default:!0}})}),s(M,B)},$$slots:{default:!0}})}),s(G,C)},$$slots:{default:!0}})}),s(e,x),Ae()}var dn=N("<!> Source Map Upload",1),un=N("<!> <!>",1),pn=N("<!> <!>",1);function mn(e,t){Se(t,!0);let n=oe(()=>xe.currentProject);const a=oe(()=>ct.getRoleForOrganization(xe.currentProject?.organizationId??0)==="readonly");var o=F(),d=E(o);{var i=l=>{Ht(l,{children:(p,f)=>{var m=pn(),v=E(m);qt(v,{children:(b,_)=>{var c=un(),g=E(c);Wt(g,{class:"flex items-center gap-2",children:(R,h)=>{var w=dn(),I=E(w);lt(I,{class:"h-5 w-5"}),U(),s(R,w)},$$slots:{default:!0}});var O=A(g,2);Yt(O,{children:(R,h)=>{U();var w=ge(`Upload source maps to see original file names and line numbers in stack traces from
				minified code.`);s(R,w)},$$slots:{default:!0}}),s(b,c)},$$slots:{default:!0}});var u=A(v,2);Zt(u,{children:(b,_)=>{ln(b,{})},$$slots:{default:!0}}),s(p,m)},$$slots:{default:!0}})};Q(d,l=>{r(n)&&!r(a)&&l(i)})}s(e,o),Ae()}var gn=N('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),_n=N('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),bn=N('<p class="pt-2 text-xs text-muted-foreground"><a target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground"> </a></p>'),fn=N('<div class="p-4"><!> <!></div>'),vn=N('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),En=N('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function jn(e,t){Se(t,!0);let n=ue(rt(ya())),a=ue(rt(Ta()));const o={bash:Fe,go:zt,javascript:Ye,typescript:St,python:ma,gradle:Ua,csharp:Ka,ruby:za,yaml:At},d=oe(()=>Re.find(h=>h.id===r(n))??Re[0]),i=oe(()=>r(d).frameworks.find(h=>h.id===r(a))?.id??r(d).frameworks[0]?.id??""),l=oe(()=>va(r(d).id,r(i),t.backendUrl,t.token));function p(h){const w=Re.find(I=>I.id===h);w&&(k(n,w.id,!0),ha(w.id))}function f(h){r(d).frameworks.some(w=>w.id===h)&&(k(a,h,!0),wa(h))}function m(h){return o[h??"bash"]}var v=En(),u=E(v),b=A(T(u),2);V(b,()=>Ue,(h,w)=>{w(h,{get value(){return r(n)},onValueChange:p,children:(I,x)=>{var K=F(),q=E(K);V(q,()=>Be,(ee,J)=>{J(ee,{class:"h-auto flex-wrap justify-start",children:(z,Z)=>{var G=F(),D=E(G);$e(D,17,()=>Re,C=>C.id,(C,$)=>{var L=F(),S=E(L);V(S,()=>Me,(M,H)=>{H(M,{get value(){return r($).id},children:(B,P)=>{U();var Y=ge();le(()=>re(Y,r($).label)),s(B,Y)},$$slots:{default:!0}})}),s(C,L)}),s(z,G)},$$slots:{default:!0}})}),s(I,K)},$$slots:{default:!0}})});var _=A(b,2);{var c=h=>{var w=gn(),I=A(E(w),2);V(I,()=>Ue,(x,K)=>{K(x,{get value(){return r(i)},onValueChange:f,children:(q,ee)=>{var J=F(),z=E(J);V(z,()=>Be,(Z,G)=>{G(Z,{class:"h-auto flex-wrap justify-start",children:(D,C)=>{var $=F(),L=E($);$e(L,17,()=>r(d).frameworks,S=>S.id,(S,M)=>{var H=F(),B=E(H);V(B,()=>Me,(P,Y)=>{Y(P,{get value(){return r(M).id},children:(se,j)=>{U();var ie=ge();le(()=>re(ie,r(M).label)),s(se,ie)},$$slots:{default:!0}})}),s(S,H)}),s(D,$)},$$slots:{default:!0}})}),s(q,J)},$$slots:{default:!0}})}),s(h,w)};Q(_,h=>{r(d).frameworks.length>1&&h(c)})}y(u);var g=A(u,2);$e(g,19,()=>r(l),h=>r(d).id+r(i)+h.title,(h,w,I)=>{var x=vn(),K=T(x),q=T(K),ee=T(q),J=T(ee,!0);y(ee);var z=A(ee,2),Z=T(z,!0);y(z),y(q);var G=A(q,2);{var D=L=>{var S=_n(),M=T(S,!0);y(S),le(()=>re(M,r(w).description)),s(L,S)};Q(G,L=>{r(w).description&&L(D)})}y(K);var C=A(K,2);{var $=L=>{var S=fn(),M=T(S);{let P=oe(()=>m(r(w).codeLanguage));Ve(M,{get code(){return r(w).code},get language(){return r(P)}})}var H=A(M,2);{var B=P=>{var Y=bn(),se=T(Y),j=T(se,!0);y(se),y(Y),le(()=>{Gt(se,"href",r(w).link.href),re(j,r(w).link.label)}),s(P,Y)};Q(H,P=>{r(w).link&&P(B)})}y(S),s(L,S)};Q(C,L=>{r(w).code&&L($)})}y(x),le(()=>{re(J,r(I)+1),re(Z,r(w).title)}),s(h,x)});var O=A(g,2);{var R=h=>{mn(h,{})};Q(O,h=>{r(n)==="nodejs"&&h(R)})}s(e,v),Ae()}var yn=N('<div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!></div>');function it(e,t){let n=ue(!1);async function a(){await navigator.clipboard.writeText(t.value),k(n,!0),setTimeout(()=>k(n,!1),2e3)}var o=yn(),d=T(o),i=T(d,!0);y(d);var l=A(d,2);me(l,{variant:"outline",size:"sm",onclick:a,children:(p,f)=>{var m=F(),v=E(m);{var u=_=>{Te(_,{class:"h-4 w-4 text-green-500"})},b=_=>{we(_,{class:"h-4 w-4"})};Q(v,_=>{r(n)?_(u):_(b,!1)})}s(p,m)},$$slots:{default:!0}}),y(o),le(()=>re(i,t.value)),s(e,o)}var hn=N('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function er(e,t){var n=hn(),a=T(n),o=A(T(a),4);it(o,{get value(){return t.endpoint}}),y(a);var d=A(a,2),i=A(T(d),2);it(i,{get value(){return t.authHeader}}),y(d);var l=A(d,2),p=A(T(l),2);Ve(p,{get code(){return t.collectorConfig},get language(){return At}}),y(l),y(n),s(e,n)}export{Jn as A,jn as O,Qn as S,er as a,Yn as b,Fe as c,qn as d,ma as e,Hn as f,Vn as g,mn as h,Wn as i,Ye as j,Zn as k,Xn as l,zn as p};
