import{c as Ue,p as Te,t as Dt,f as tt}from"./v8eObPjy.js";import"./DsnmJJEf.js";import{p as ve,c as X,f as T,k as x,l as J,t as be,b as i,m as p,a as he,r as h,v as E,o as ne,g as r,q as oe,u as U,j as ge,e as Ae,an as Bt,h as Ve}from"./DdLJ-Jq-.js";import{c as ee}from"./_2wP5a9n.js";import{a as Ie,b as Re,T as Me}from"./DYGbAFtr.js";import{u as Qe,a as Ut}from"./Di5Uf8kx.js";import{i as te}from"./Bec8tFyx.js";import{e as Oe,i as Ft}from"./CQepFboV.js";import{C as Fe}from"./ZAzC_aU_.js";import{a as Kt,d as Gt}from"./BmnNfo79.js";import{a as Ht}from"./BPNc1Q-4.js";import{H as zt,g as qt}from"./BROuxBV9.js";import{C as Wt}from"./GVynZ3_D.js";import{C as Zt}from"./BxY7jcAd.js";import{C as Yt}from"./DzDOZz8C.js";import{C as Xt,a as Vt}from"./CO2CfyrX.js";import{B as Ne}from"./txoAjGfg.js";import{L as Qt}from"./BLINML1I.js";import{A as jt,a as Jt,b as ea,c as ta,d as aa,e as ra}from"./DYpLdg8V.js";import{R as na}from"./CuKLpQN5.js";import{K as at}from"./PuhgQF_k.js";import{C as je}from"./B3QR6Zpd.js";import{p as oa}from"./SnhvkPqy.js";import{t as sa}from"./BSuxCYQD.js";const Je="[A-Za-z$_][0-9A-Za-z$_]*",ia=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],ca=["true","false","null","undefined","NaN","Infinity"],rt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],nt=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],ot=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],la=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],da=[].concat(ot,rt,nt);function ua(e){const t=e.regex,n=(u,{after:R})=>{const k="</"+u[0].slice(1);return u.input.indexOf(k,R)!==-1},a=Je,s={begin:"<>",end:"</>"},d=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(u,R)=>{const k=u[0].length+u.index,z=u.input[k];if(z==="<"||z===","){R.ignoreMatch();return}z===">"&&(n(u,{after:k})||R.ignoreMatch());let q;const W=u.input.substring(k);if(q=W.match(/^\s*=/)){R.ignoreMatch();return}if((q=W.match(/^\s+extends\s+/))&&q.index===0){R.ignoreMatch();return}}},c={$pattern:Je,keyword:ia,literal:ca,built_in:da,"variable.language":la},_="[0-9](_?[0-9])*",m=`\\.(${_})`,v="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",S={className:"number",variants:[{begin:`(\\b(${v})((${m})|\\.)?|(${m}))[eE][+-]?(${_})\\b`},{begin:`\\b(${v})\\b((${m})\\b|\\.)?|(${m})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},l={className:"subst",begin:"\\$\\{",end:"\\}",keywords:c,contains:[]},g={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"xml"}},w={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"css"}},f={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"graphql"}},I={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,l]},N={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},b=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,w,f,I,{match:/\$\d+/},S];l.contains=b.concat({begin:/\{/,end:/\}/,keywords:c,contains:["self"].concat(b)});const y=[].concat(N,l.contains),O=y.concat([{begin:/(\s*)\(/,end:/\)/,keywords:c,contains:["self"].concat(y)}]),A={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:c,contains:O},K={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},$={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...rt,...nt]}},V={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},ae={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[A],illegal:/%/},se={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(u){return t.concat("(?!",u.join("|"),")")}const G={match:t.concat(/\b/,ce([...ot,"super","import"].map(u=>`${u}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},P={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},C={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},A]},D="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",B={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead(D)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[A]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:c,exports:{PARAMS_CONTAINS:O,CLASS_REFERENCE:$},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),V,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,w,f,I,N,{match:/\$\d+/},S,$,{scope:"attr",match:a+t.lookahead(":"),relevance:0},B,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[N,e.REGEXP_MODE,{className:"function",begin:D,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:c,contains:O}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:s.begin,end:s.end},{match:d},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},ae,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[A,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},P,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[A]},G,se,K,C,{match:/\$[(.]/}]}}const Ce={name:"javascript",register:ua};function pa(e){const t=e.regex,n={},a={begin:/\$\{/,end:/\}/,contains:["self",{begin:/:-/,contains:[n]}]};Object.assign(n,{className:"variable",variants:[{begin:t.concat(/\$[\w\d#@][\w\d_]*/,"(?![\\w\\d])(?![$])")},a]});const s={className:"subst",begin:/\$\(/,end:/\)/,contains:[e.BACKSLASH_ESCAPE]},d=e.inherit(e.COMMENT(),{match:[/(^|\s)/,/#.*$/],scope:{2:"comment"}}),o={begin:/<<-?\s*(?=\w+)/,starts:{contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,className:"string"})]}},c={className:"string",begin:/"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,n,s]};s.contains.push(c);const _={match:/\\"/},m={className:"string",begin:/'/,end:/'/},v={match:/\\'/},S={begin:/\$?\(\(/,end:/\)\)/,contains:[{begin:/\d+#[0-9a-f]+/,className:"number"},e.NUMBER_MODE,n]},l=["fish","bash","zsh","sh","csh","ksh","tcsh","dash","scsh"],g=e.SHEBANG({binary:`(${l.join("|")})`,relevance:10}),w={className:"function",begin:/\w[\w\d_]*\s*\(\s*\)\s*\{/,returnBegin:!0,contains:[e.inherit(e.TITLE_MODE,{begin:/\w[\w\d_]*/})],relevance:0},f=["if","then","else","elif","fi","time","for","while","until","in","do","done","case","esac","coproc","function","select"],I=["true","false"],M={match:/(\/[a-z._-]+)+/},N=["break","cd","continue","eval","exec","exit","export","getopts","hash","pwd","readonly","return","shift","test","times","trap","umask","unset"],b=["alias","bind","builtin","caller","command","declare","echo","enable","help","let","local","logout","mapfile","printf","read","readarray","source","sudo","type","typeset","ulimit","unalias"],y=["autoload","bg","bindkey","bye","cap","chdir","clone","comparguments","compcall","compctl","compdescribe","compfiles","compgroups","compquote","comptags","comptry","compvalues","dirs","disable","disown","echotc","echoti","emulate","fc","fg","float","functions","getcap","getln","history","integer","jobs","kill","limit","log","noglob","popd","print","pushd","pushln","rehash","sched","setcap","setopt","stat","suspend","ttyctl","unfunction","unhash","unlimit","unsetopt","vared","wait","whence","where","which","zcompile","zformat","zftp","zle","zmodload","zparseopts","zprof","zpty","zregexparse","zsocket","zstyle","ztcp"],O=["chcon","chgrp","chown","chmod","cp","dd","df","dir","dircolors","ln","ls","mkdir","mkfifo","mknod","mktemp","mv","realpath","rm","rmdir","shred","sync","touch","truncate","vdir","b2sum","base32","base64","cat","cksum","comm","csplit","cut","expand","fmt","fold","head","join","md5sum","nl","numfmt","od","paste","ptx","pr","sha1sum","sha224sum","sha256sum","sha384sum","sha512sum","shuf","sort","split","sum","tac","tail","tr","tsort","unexpand","uniq","wc","arch","basename","chroot","date","dirname","du","echo","env","expr","factor","groups","hostid","id","link","logname","nice","nohup","nproc","pathchk","pinky","printenv","printf","pwd","readlink","runcon","seq","sleep","stat","stdbuf","stty","tee","test","timeout","tty","uname","unlink","uptime","users","who","whoami","yes"];return{name:"Bash",aliases:["sh","zsh"],keywords:{$pattern:/\b[a-z][a-z0-9._-]+\b/,keyword:f,literal:I,built_in:[...N,...b,"set","shopt",...y,...O]},contains:[g,e.SHEBANG(),w,S,d,o,M,c,_,m,v,n]}}const fe={name:"bash",register:pa};function ma(e){const t=e.regex,n=/(?![A-Za-z0-9])(?![$])/,a=t.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,n),s=t.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,n),d=t.concat(/[A-Z]+/,n),o={scope:"variable",match:"\\$+"+a},c={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},_={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},m=e.inherit(e.APOS_STRING_MODE,{illegal:null}),v=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(_)}),S={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(_),"on:begin":(P,C)=>{C.data._beginMatch=P[1]||P[2]},"on:end":(P,C)=>{C.data._beginMatch!==P[1]&&C.ignoreMatch()}},l=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),g=`[ 	
]`,w={scope:"string",variants:[v,m,S,l]},f={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},I=["false","null","true"],M=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],N=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],y={keyword:M,literal:(P=>{const C=[];return P.forEach(D=>{C.push(D),D.toLowerCase()===D?C.push(D.toUpperCase()):C.push(D.toLowerCase())}),C})(I),built_in:N},O=P=>P.map(C=>C.replace(/\|\d+$/,"")),A={variants:[{match:[/new/,t.concat(g,"+"),t.concat("(?!",O(N).join("\\b|"),"\\b)"),s],scope:{1:"keyword",4:"title.class"}}]},K=t.concat(a,"\\b(?!\\()"),$={variants:[{match:[t.concat(/::/,t.lookahead(/(?!class\b)/)),K],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[s,t.concat(/::/,t.lookahead(/(?!class\b)/)),K],scope:{1:"title.class",3:"variable.constant"}},{match:[s,t.concat("::",t.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[s,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},V={scope:"attr",match:t.concat(a,t.lookahead(":"),t.lookahead(/(?!::)/))},ae={relevance:0,begin:/\(/,end:/\)/,keywords:y,contains:[V,o,$,e.C_BLOCK_COMMENT_MODE,w,f,A]},se={relevance:0,match:[/\b/,t.concat("(?!fn\\b|function\\b|",O(M).join("\\b|"),"|",O(N).join("\\b|"),"\\b)"),a,t.concat(g,"*"),t.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[ae]};ae.contains.push(se);const ce=[V,$,e.C_BLOCK_COMMENT_MODE,w,f,A],G={begin:t.concat(/#\[\s*\\?/,t.either(s,d)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:I,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:I,keyword:["new","array"]},contains:["self",...ce]},...ce,{scope:"meta",variants:[{match:s},{match:d}]}]};return{case_insensitive:!1,keywords:y,contains:[G,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},c,{scope:"variable.language",match:/\$this\b/},o,se,$,{match:[/const/,/\s/,a],scope:{1:"keyword",3:"variable.constant"}},A,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:y,contains:["self",G,o,$,e.C_BLOCK_COMMENT_MODE,w,f]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},w,f]}}const Zr={name:"php",register:ma};function ga(e){const t=e.regex,n=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),a=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],c={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:a,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},_={className:"meta",begin:/^(>>>|\.\.\.) /},m={className:"subst",begin:/\{/,end:/\}/,keywords:c,illegal:/#/},v={begin:/\{\{/,relevance:0},S={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,_],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,_],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,_,v,m]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,_,v,m]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,v,m]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,v,m]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},l="[0-9](_?[0-9])*",g=`(\\b(${l}))?\\.(${l})|\\b(${l})\\.`,w=`\\b|${a.join("|")}`,f={className:"number",relevance:0,variants:[{begin:`(\\b(${l})|(${g}))[eE][+-]?(${l})[jJ]?(?=${w})`},{begin:`(${g})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${w})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${w})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${w})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${w})`},{begin:`\\b(${l})[jJ](?=${w})`}]},I={className:"comment",begin:t.lookahead(/# type:/),end:/$/,keywords:c,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},M={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:c,contains:["self",_,f,S,e.HASH_COMMENT_MODE]}]};return m.contains=[S,f,_],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:c,illegal:/(<\/|\?)|=>/,contains:[_,f,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},S,I,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[M]},{variants:[{match:[/\bclass/,/\s+/,n,/\s*/,/\(\s*/,n,/\s*\)/]},{match:[/\bclass/,/\s+/,n]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[f,M,S]}]}}const ba={name:"python",register:ga};function Yr(e){const t="go get go.tracewayapp.com";switch(e){case"gin":return`${t} && go get go.tracewayapp.com/tracewaygin`;case"chi":return`${t} && go get go.tracewayapp.com/tracewaychi`;case"fiber":return`${t} && go get go.tracewayapp.com/tracewayfiber`;case"fasthttp":return`${t} && go get go.tracewayapp.com/tracewayfasthttp`;case"stdlib":return`${t} && go get go.tracewayapp.com/tracewayhttp`;case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.1")';case"ios":return'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';default:return t}}function Xr(e,t,n){const a=t?`${t}@${n}/api/report`:`YOUR_TOKEN@${n}/api/report`;switch(e){case"gin":return`package main

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
// .env  - point the OTLP exporter at Traceway
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
# OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer%20${t||"YOUR_TOKEN"}
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
}`;case"ios":return`import SwiftUI
import Traceway

@main
struct MyApp: App {
    init() {
        Traceway.start(
            connectionString: "${a}",
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
        "${a}",
        traceway.WithVersion("1.0.0"),
        traceway.WithServerName("my-server"),
    )
}`}}function Vr(e){return e==="symfony"?`<?php
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
fatalError("Test error from Traceway integration")`:e&&Ue(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:`r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`}function Qr(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

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
}`;if(e&&Ue(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

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
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${_a(e)}";

captureException(new Error("Test error"));`}return`r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`}function _a(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function jr(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android",ios:"iOS"}[e]||e}function Jr(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||e==="ios"||Ue(e)?"javascript":"go"}const Ee=[{id:"collector",label:"Collector",frameworks:[]},{id:"nodejs",label:"Node.js",frameworks:[{id:"express",label:"Express"},{id:"nestjs",label:"NestJS"},{id:"fastify",label:"Fastify"},{id:"nextjs",label:"Next.js"},{id:"koa",label:"Koa"},{id:"other",label:"Other"}]},{id:"go",label:"Go",frameworks:[{id:"gin",label:"Gin"},{id:"echo",label:"Echo"},{id:"chi",label:"Chi"},{id:"fiber",label:"Fiber"},{id:"mux",label:"gorilla/mux"},{id:"nethttp",label:"net/http"}]},{id:"python",label:"Python",frameworks:[{id:"django",label:"Django"},{id:"flask",label:"Flask"},{id:"fastapi",label:"FastAPI"},{id:"other",label:"Other"}]},{id:"java",label:"Java",frameworks:[{id:"agent",label:"Any framework"},{id:"spring",label:"Spring Boot"}]},{id:"dotnet",label:".NET",frameworks:[]},{id:"php",label:"PHP",frameworks:[{id:"symfony",label:"Symfony"},{id:"laravel",label:"Laravel"},{id:"slim",label:"Slim"},{id:"other",label:"Other"}]},{id:"ruby",label:"Ruby",frameworks:[{id:"rails",label:"Rails"},{id:"other",label:"Other"}]},{id:"other",label:"Other",frameworks:[]}];function st(e,t,n=[]){return["OTEL_SERVICE_NAME=my-service",`OTEL_EXPORTER_OTLP_ENDPOINT=${e}/api/otel`,`OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer ${t}`,...n].join(`
`)}function me(e,t,n=[],a=""){return{title:"Configure the Exporter",description:`Set these environment variables in your shell, .env file, or deployment config. The SDK appends /v1/traces and /v1/metrics to the endpoint automatically.${a?" "+a:""}`,code:st(e,t,n),codeLanguage:"bash"}}function fa(e,t){return`exporters:
  otlphttp:
    endpoint: "${e}/api/otel"
    headers:
      Authorization: "Bearer ${t}"

service:
  pipelines:
    traces:
      exporters: [otlphttp]
    metrics:
      exporters: [otlphttp]`}const ya=`import (
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
}`,et={gin:{lib:"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin",snippet:`import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

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
http.ListenAndServe(":8080", mux)`,note:"Wrap each route individually with Go 1.22+ method patterns so the route is set on spans and endpoints group by pattern instead of raw URL."}},Ea="npm install @opentelemetry/api @opentelemetry/auto-instrumentations-node";function Be(e,t,n,a,s){return[{title:"Install the SDK",description:a,code:Ea,codeLanguage:"bash"},me(e,t),{title:"Run with Instrumentation",description:s,code:`node --require @opentelemetry/auto-instrumentations-node/register ${n}`,codeLanguage:"bash"}]}function va(e,t,n,a){switch(e){case"collector":return[{title:"Add the Traceway Exporter",description:"Merge this into your OpenTelemetry Collector configuration. Any pipeline that lists the otlphttp exporter will be forwarded to Traceway.",code:fa(n,a),codeLanguage:"yaml"},{title:"Restart the Collector",description:"Restart the Collector to apply the configuration. Traces and metrics flowing through its pipelines will appear in Traceway."}];case"nodejs":return t==="fastify"?[{title:"Install the SDK",description:"Fastify is instrumented by the @fastify/otel package maintained by the Fastify team.",code:"npm install @opentelemetry/api @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @fastify/otel",codeLanguage:"bash"},{title:"Create instrumentation.js",description:"Add this file at the project root.",code:`const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { FastifyOtelInstrumentation } = require('@fastify/otel');

new NodeSDK({
  instrumentations: [
    getNodeAutoInstrumentations(),
    new FastifyOtelInstrumentation({ registerOnInitialization: true }),
  ],
}).start();`,codeLanguage:"javascript"},me(n,a),{title:"Run with Instrumentation",code:"node --require ./instrumentation.js app.js",codeLanguage:"bash"}]:t==="nextjs"?[{title:"Install the SDK",code:"npm install @vercel/otel",codeLanguage:"bash"},{title:"Create instrumentation.ts",description:"Add this file at the project root (next to package.json). Next.js calls register() automatically on startup.",code:`import { registerOTel } from '@vercel/otel'

export function register() {
  registerOTel({ serviceName: 'my-service' })
}`,codeLanguage:"typescript"},me(n,a,[],"Start your app normally with next start; no extra flags are needed.")]:t==="nestjs"?Be(n,a,"dist/main.js","Auto-instrumentation captures NestJS routes, status codes, and errors through the default Express adapter with no code changes. If you use the Fastify adapter, follow the Fastify setup instead.","Routes group by pattern automatically."):t==="koa"?Be(n,a,"app.js","Auto-instrumentation captures Koa requests, status codes, and errors with no code changes.","Route patterns are captured when routing with @koa/router."):Be(n,a,"app.js","Auto-instrumentation captures routes, status codes, and errors with no code changes.","For ESM apps, add --experimental-loader=@opentelemetry/instrumentation/hook.mjs and use --import instead of --require.");case"go":{const s=et[t]??et.gin;return[{title:"Install the SDK",code:`go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp ${s.lib}`,codeLanguage:"bash"},{title:"Initialize the SDK",description:"Call initTracer at startup and defer tp.Shutdown(ctx) before exit. The exporter reads the environment variables from the next step.",code:ya,codeLanguage:"go"},{title:"Add the Middleware",description:s.note,code:s.snippet,codeLanguage:"go"},me(n,a)]}case"python":{const s={django:{cmd:"opentelemetry-instrument python manage.py runserver --noreload",note:"The --noreload flag is required with runserver; the autoreloader breaks instrumentation. It is not needed under gunicorn or other production servers."},flask:{cmd:"opentelemetry-instrument flask run"},fastapi:{cmd:"opentelemetry-instrument uvicorn main:app",note:"Avoid --reload and --workers with zero-code instrumentation; for multi-worker production use gunicorn with uvicorn workers."},other:{cmd:"opentelemetry-instrument python app.py"}},d=s[t]??s.other;return[{title:"Install the SDK",description:"opentelemetry-bootstrap detects your installed packages and adds the matching instrumentation.",code:`pip install opentelemetry-distro opentelemetry-exporter-otlp-proto-http
opentelemetry-bootstrap -a install`,codeLanguage:"bash"},me(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the Python SDK defaults to gRPC."),{title:"Run with Instrumentation",description:d.note,code:d.cmd,codeLanguage:"bash"}]}case"java":return t==="spring"?[{title:"Add the Starter",description:"Add the OpenTelemetry Spring Boot starter to your Gradle build (a Maven dependency works the same way).",code:`implementation(platform("io.opentelemetry.instrumentation:opentelemetry-instrumentation-bom:2.28.1"))
implementation("io.opentelemetry.instrumentation:opentelemetry-spring-boot-starter")`,codeLanguage:"gradle"},me(n,a,[],"Start your app normally; the starter reads these variables and reports routes, status codes, and exceptions.")]:[{title:"Download the Java Agent",description:"The agent instruments Spring, JAX-RS, and most Java frameworks with zero code changes.",code:"curl -L -O https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar",codeLanguage:"bash"},me(n,a),{title:"Run with the Agent",code:"java -javaagent:./opentelemetry-javaagent.jar -jar myapp.jar",codeLanguage:"bash"}];case"dotnet":return[{title:"Install the Packages",code:`dotnet add package OpenTelemetry.Extensions.Hosting
dotnet add package OpenTelemetry.Instrumentation.AspNetCore
dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol`,codeLanguage:"bash"},{title:"Add to Program.cs",description:"Keep AddOtlpExporter() empty so the exporter is driven entirely by the environment variables in the next step.",code:`builder.Services.AddOpenTelemetry()
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddOtlpExporter());`,codeLanguage:"csharp"},me(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the .NET exporter defaults to gRPC.")];case"php":{const d={symfony:" open-telemetry/opentelemetry-auto-symfony",laravel:" open-telemetry/opentelemetry-auto-laravel",slim:" open-telemetry/opentelemetry-auto-slim",other:""}[t]??"";return[{title:"Install the SDK",description:"Auto-instrumentation needs the opentelemetry PECL extension; enable it with extension=opentelemetry in php.ini."+(t==="other"?" Find auto-instrumentation packages for your framework in the OpenTelemetry registry.":""),code:`pecl install opentelemetry
composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter${d}`,codeLanguage:"bash",link:t==="other"?{label:"Browse PHP instrumentation packages",href:"https://opentelemetry.io/ecosystem/registry/?language=php&component=instrumentation"}:void 0},me(n,a,["OTEL_PHP_AUTOLOAD_ENABLED=true"],"These must be real process environment variables; the extension does not read framework .env files. Use env[...] in php-fpm pool config or SetEnv in Apache.")]}case"ruby":return t==="rails"?[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-rails",codeLanguage:"bash"},{title:"Create the Initializer",description:"Add config/initializers/opentelemetry.rb.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/rails'

OpenTelemetry::SDK.configure do |c|
  c.use 'OpenTelemetry::Instrumentation::Rails'
end`,codeLanguage:"ruby"},me(n,a)]:[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-all",codeLanguage:"bash"},{title:"Configure the SDK",description:"Run this once at startup, before your app starts handling requests.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure do |c|
  c.use_all
end`,codeLanguage:"ruby"},me(n,a)];default:return[{title:"Configure any OpenTelemetry SDK",description:"Any language with an OTLP/HTTP exporter works. Set these environment variables; the protocol variable matters for SDKs that default to gRPC. Make sure http.route is set on root server spans so endpoints group by route pattern, and use SpanKind CONSUMER for background jobs.",code:st(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"]),codeLanguage:"bash",link:{label:"View all supported languages",href:"https://opentelemetry.io/docs/languages/"}}]}}const it="traceway_setup_mode",ct="traceway_otel_language",lt="traceway_otel_framework";function en(){try{const e=localStorage.getItem(it);if(e==="ai"||e==="manual")return e}catch{return"ai"}return"ai"}function ha(e){try{localStorage.setItem(it,e)}catch{return}}function Ta(){try{const e=localStorage.getItem(ct);if(e&&Ee.some(t=>t.id===e))return e}catch{return Ee[0].id}return Ee[0].id}function wa(e){try{localStorage.setItem(ct,e)}catch{return}}function Sa(){try{return localStorage.getItem(lt)}catch{return null}return null}function Aa(e){try{localStorage.setItem(lt,e)}catch{return}}var Oa=x("<!> <!>",1);function tn(e,t){ve(t,!0);function n(d){(d==="ai"||d==="manual")&&(ha(d),t.onModeChange(d))}var a=X(),s=T(a);ee(s,()=>Me,(d,o)=>{o(d,{get value(){return t.mode},onValueChange:n,children:(c,_)=>{var m=X(),v=T(m);ee(v,()=>Ie,(S,l)=>{l(S,{get class(){return Ut},children:(g,w)=>{var f=Oa(),I=T(f);ee(I,()=>Re,(N,b)=>{b(N,{value:"ai",get class(){return Qe},children:(y,O)=>{J();var A=be("AI");i(y,A)},$$slots:{default:!0}})});var M=p(I,2);ee(M,()=>Re,(N,b)=>{b(N,{value:"manual",get class(){return Qe},children:(y,O)=>{J();var A=be("Manual");i(y,A)},$$slots:{default:!0}})}),i(g,f)},$$slots:{default:!0}})}),i(c,m)},$$slots:{default:!0}})}),i(e,a),he()}const Ra="npx skills add tracewayapp/traceway";function xa(e,t,n=null){const a=[{text:"/traceway-setup with token ",bold:!1},{text:t,bold:!0},{text:" and url ",bold:!1},{text:e,bold:!0}];return n&&a.push({text:" and source map upload token ",bold:!1},{text:n,bold:!0}),a}var Na=x('<div class="flex items-start gap-2"><div><!></div> <!></div>');function pe(e,t){ve(t,!0);let n=oa(t,"wrap",3,!1);var a=Na(),s=h(a),d=h(s);zt(d,{get language(){return t.language},get code(){return t.code}}),E(s);var o=p(s,2);Fe(o,{get text(){return t.code}}),E(a),ne(()=>Kt(s,1,`min-w-0 flex-1 overflow-x-auto rounded-md text-sm ${n()?"wrap-code":""} ${sa.isDark?"dark-code":"light-code"}`)),i(e,a),he()}var Ca=x('<span class="break-all text-muted-foreground"> </span>'),Ia=x('<div class="flex items-start gap-2"><code class="block min-w-0 flex-1 rounded-md bg-muted px-4 py-3 font-mono text-sm break-words whitespace-pre-wrap text-foreground"></code> <!></div>');function Ma(e,t){ve(t,!0);const n=U(()=>t.parts.map(o=>o.text).join(""));var a=Ia(),s=h(a);Oe(s,21,()=>t.parts,Ft,(o,c)=>{var _=X(),m=T(_);{var v=l=>{var g=Ca(),w=h(g,!0);E(g),ne(()=>oe(w,r(c).text)),i(l,g)},S=l=>{var g=be();ne(()=>oe(g,r(c).text)),i(l,g)};te(m,l=>{r(c).bold?l(v):l(S,!1)})}i(o,_)}),E(s);var d=p(s,2);Fe(d,{get text(){return r(n)},label:"Copy"}),E(a),i(e,a),he()}var La=x(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function an(e,t){ve(t,!0);const n=U(()=>Te.currentProject?.sourceMapToken??null),a=U(()=>xa(t.backendUrl,t.token,r(n)));var s=La(),d=T(s),o=p(h(d),2),c=h(o);pe(c,{get code(){return Ra},get language(){return fe}}),E(o),E(d);var _=p(d,2),m=h(_),v=p(h(m),2),S=h(v);E(v),E(m);var l=p(m,2),g=h(l);Ma(g,{get parts(){return r(a)}}),E(l),E(_),ne(()=>oe(S,`Paste this prompt into your agent. Your instance URL and project token are already filled in${r(n)?", along with your source map upload token":""}.`)),i(e,s),he()}const Le="[A-Za-z$_][0-9A-Za-z$_]*",dt=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],ut=["true","false","null","undefined","NaN","Infinity"],pt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],mt=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],gt=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],bt=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],_t=[].concat(gt,pt,mt);function $a(e){const t=e.regex,n=(u,{after:R})=>{const k="</"+u[0].slice(1);return u.input.indexOf(k,R)!==-1},a=Le,s={begin:"<>",end:"</>"},d=/<[A-Za-z0-9\\._:-]+\s*\/>/,o={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(u,R)=>{const k=u[0].length+u.index,z=u.input[k];if(z==="<"||z===","){R.ignoreMatch();return}z===">"&&(n(u,{after:k})||R.ignoreMatch());let q;const W=u.input.substring(k);if(q=W.match(/^\s*=/)){R.ignoreMatch();return}if((q=W.match(/^\s+extends\s+/))&&q.index===0){R.ignoreMatch();return}}},c={$pattern:Le,keyword:dt,literal:ut,built_in:_t,"variable.language":bt},_="[0-9](_?[0-9])*",m=`\\.(${_})`,v="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",S={className:"number",variants:[{begin:`(\\b(${v})((${m})|\\.)?|(${m}))[eE][+-]?(${_})\\b`},{begin:`\\b(${v})\\b((${m})\\b|\\.)?|(${m})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},l={className:"subst",begin:"\\$\\{",end:"\\}",keywords:c,contains:[]},g={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"xml"}},w={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"css"}},f={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,l],subLanguage:"graphql"}},I={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,l]},N={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},b=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,w,f,I,{match:/\$\d+/},S];l.contains=b.concat({begin:/\{/,end:/\}/,keywords:c,contains:["self"].concat(b)});const y=[].concat(N,l.contains),O=y.concat([{begin:/(\s*)\(/,end:/\)/,keywords:c,contains:["self"].concat(y)}]),A={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:c,contains:O},K={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},$={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...pt,...mt]}},V={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},ae={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[A],illegal:/%/},se={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function ce(u){return t.concat("(?!",u.join("|"),")")}const G={match:t.concat(/\b/,ce([...gt,"super","import"].map(u=>`${u}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},P={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},C={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},A]},D="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",B={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead(D)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[A]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:c,exports:{PARAMS_CONTAINS:O,CLASS_REFERENCE:$},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),V,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,g,w,f,I,N,{match:/\$\d+/},S,$,{scope:"attr",match:a+t.lookahead(":"),relevance:0},B,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[N,e.REGEXP_MODE,{className:"function",begin:D,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:c,contains:O}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:s.begin,end:s.end},{match:d},{begin:o.begin,"on:begin":o.isTrulyOpeningTag,end:o.end}],subLanguage:"xml",contains:[{begin:o.begin,end:o.end,skip:!0,contains:["self"]}]}]},ae,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[A,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},P,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[A]},G,se,K,C,{match:/\$[(.]/}]}}function Pa(e){const t=e.regex,n=$a(e),a=Le,s=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],d={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},o={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:s},contains:[n.exports.CLASS_REFERENCE]},c={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},_=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],m={$pattern:Le,keyword:dt.concat(_),literal:ut,built_in:_t.concat(s),"variable.language":bt},v={className:"meta",begin:"@"+a},S=(f,I,M)=>{const N=f.contains.findIndex(b=>b.label===I);if(N===-1)throw new Error("can not find mode to replace");f.contains.splice(N,1,M)};Object.assign(n.keywords,m),n.exports.PARAMS_CONTAINS.push(v);const l=n.contains.find(f=>f.scope==="attr"),g=Object.assign({},l,{match:t.concat(a,t.lookahead(/\s*\?:/))});n.exports.PARAMS_CONTAINS.push([n.exports.CLASS_REFERENCE,l,g]),n.contains=n.contains.concat([v,d,o,g]),S(n,"shebang",e.SHEBANG()),S(n,"use_strict",c);const w=n.contains.find(f=>f.label==="func.def");return w.relevance=0,Object.assign(n,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),n}const ft={name:"typescript",register:Pa};function ka(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const Da={name:"gradle",register:ka};function Ba(e){const t=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],n=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],a=["default","false","null","true"],s=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],d=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],o={keyword:s.concat(d),built_in:t,literal:a},c=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),_={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},m={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},v={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},S=e.inherit(v,{illegal:/\n/}),l={className:"subst",begin:/\{/,end:/\}/,keywords:o},g=e.inherit(l,{illegal:/\n/}),w={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,g]},f={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},l]},I=e.inherit(f,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},g]});l.contains=[f,w,v,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,_,e.C_BLOCK_COMMENT_MODE],g.contains=[I,w,S,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,_,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const M={variants:[m,f,w,v,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},N={begin:"<",end:">",contains:[{beginKeywords:"in out"},c]},b=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",y={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:o,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},M,_,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},c,N,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[c,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[c,N,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+b+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:o,contains:[{beginKeywords:n.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,N],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,relevance:0,contains:[M,_,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},y]}}const Ua={name:"csharp",register:Ba};function Fa(e){const t=e.regex,n="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",a=t.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),s=t.concat(a,/(::\w+)*/),o={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},c={className:"doctag",begin:"@[A-Za-z]+"},_={begin:"#<",end:">"},m=[e.COMMENT("#","$",{contains:[c]}),e.COMMENT("^=begin","^=end",{contains:[c],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],v={className:"subst",begin:/#\{/,end:/\}/,keywords:o},S={className:"string",contains:[e.BACKSLASH_ESCAPE,v],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:t.concat(/<<[-~]?'?/,t.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,v]})]}]},l="[1-9](_?[0-9])*|0",g="[0-9](_?[0-9])*",w={className:"number",relevance:0,variants:[{begin:`\\b(${l})(\\.(${g}))?([eE][+-]?(${g})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},f={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:o}]},A=[S,{variants:[{match:[/class\s+/,s,/\s+<\s+/,s]},{match:[/\b(class|module)\s+/,s]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:o},{match:[/(include|extend)\s+/,s],scope:{2:"title.class"},keywords:o},{relevance:0,match:[s,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:a,scope:"title.class"},{match:[/def/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[f]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[S,{begin:n}],relevance:0},w,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:o},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,v],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(_,m),relevance:0}].concat(_,m);v.contains=A,f.contains=A;const ae=[{begin:/^\s*=>/,starts:{end:"$",contains:A}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:o,contains:A}}];return m.unshift(_),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:o,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(ae).concat(m).concat(A)}}const Ka={name:"ruby",register:Fa};function Ga(e){const t="true false yes no null",n="[\\w#;/?:@&=+$,.~*'()[\\]]+",a={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},s={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},d={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},o={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,s]},c=e.inherit(o,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),l={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},g={end:",",endsWithParent:!0,excludeEnd:!0,keywords:t,relevance:0},w={begin:/\{/,end:/\}/,contains:[g],illegal:"\\n",relevance:0},f={begin:"\\[",end:"\\]",contains:[g],illegal:"\\n",relevance:0},I=[a,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+n},{className:"type",begin:"!<"+n+">"},{className:"type",begin:"!"+n},{className:"type",begin:"!!"+n},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:t,keywords:{literal:t}},l,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},w,f,d,o],M=[...I];return M.pop(),M.push(c),g.contains=M,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:I}}const yt={name:"yaml",register:Ga};var Ha=x("<!> Regenerate",1),za=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p> <!> <p class="mt-2 text-xs text-muted-foreground">This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code> instead of the flag.</p></div>`,1),qa=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p> <!> <p class="mt-2 text-xs text-muted-foreground">Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p> <!> <p class="mt-2 text-xs text-muted-foreground">Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.</p></div>`,1),Wa=x(`<div><p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p> <!> <p class="mt-2 text-xs text-muted-foreground">Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin
					embeds a ProGuard UUID into BuildConfig (matching Honeycomb's <code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping <code class="font-mono">&lt;uuid&gt;.txt</code>.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p> <!> <p class="mt-2 text-xs text-muted-foreground">Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native <code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so re-upload
					on each release.</p></div>`,1),Za=x('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <!></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <!></div>',1),Ya=x('<!> <div><p class="mb-2 text-sm font-medium"> </p> <!></div>',1),Xa=x('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!> <!></div></div> <!></div>'),Va=x('<p class="text-sm text-muted-foreground"> </p>'),Qa=x('<p class="text-sm text-muted-foreground">Plain release builds already report readable traces. Only obfuscated builds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code>) need this: generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code> after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/flutter" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Flutter docs</a></p>'),ja=x(`<p class="text-sm text-muted-foreground">Release crashes report against stripped machine code. Generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/ios" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">iOS docs</a></p>`),Ja=x(`<p class="text-sm text-muted-foreground">Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code> and native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries
				after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/symbolicator/android" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Android docs</a></p>`),er=x('<p class="text-sm text-muted-foreground"> </p>'),tr=x("<!> Generating...",1),ar=x("<!> Generate Upload Token",1),rr=x('<div class="flex items-center justify-between gap-4"><!> <!></div>'),nr=x("<!> <!>",1),or=x("<!> <!>",1),sr=x(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1),ir=x("<!> <!>",1);function cr(e,t){ve(t,!0);const n={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:ft,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:Ce,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:Ce,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let a=Ae("vite"),s=Ae(!1);const d="npm install -D @tracewayapp/bundler-plugin",o=U(()=>Te.currentProject),c=U(()=>r(o)?.sourceMapToken??null),_=U(()=>tt(r(o))),m=U(()=>r(o)?.framework==="flutter"),v=U(()=>r(o)?.framework==="ios"),S=U(()=>r(o)?.framework==="android"),l=U(()=>r(m)||r(v)||r(S)?"debug symbols":"source maps"),g=U(()=>r(o)?.framework!=="react-native"),w=U(()=>r(o)&&r(c)?`npx @tracewayapp/sourcemap-upload \\
  --url ${r(o).backendUrl} \\
  --token ${r(c)} \\
  --directory ${r(g)?n[r(a)].directory:"dist"}`:""),f="flutter build apk --release --obfuscate --split-debug-info=build/symbols",I=U(()=>r(o)&&r(c)?`dart run traceway:upload_symbols \\
  --token ${r(c)} \\
  --url ${r(o).backendUrl}`:""),M=`xcodebuild -scheme MyApp -configuration Release \\
  -archivePath build/MyApp.xcarchive archive`,N=U(()=>r(o)&&r(c)?`curl -X POST ${r(o).backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${r(c)}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`:""),b=U(()=>r(o)&&r(c)?`plugins {
  id("com.tracewayapp.symbols")
}

android {
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  token = "${r(c)}"
  url = "${r(o).backendUrl}"
}`:""),y="./gradlew assembleRelease uploadReleaseTracewaySymbols";let O=Ae(!1);async function A(){ge(s,!0);try{await Te.generateSourceMapToken()}finally{ge(s,!1)}}async function K(){ge(s,!0);try{await Te.generateSourceMapToken(),ge(O,!1),Dt.success("Successfully regenerated the Upload Token")}finally{ge(s,!1)}}var $=ir(),V=T($);{var ae=G=>{var P=Xa(),C=h(P),D=p(h(C),2),B=h(D),u=h(B,!0);E(B);var R=p(B,2);{let L=U(()=>r(c)??"");Fe(R,{get text(){return r(L)}})}var k=p(R,2);Ne(k,{variant:"destructiveOutline",size:"sm",onclick:()=>ge(O,!0),children:(L,re)=>{var H=Ha(),le=T(H);na(le,{class:"mr-2 h-4 w-4"}),J(),i(L,H)},$$slots:{default:!0}}),E(D),E(C);var z=p(C,2);{var q=L=>{var re=za(),H=T(re),le=p(h(H),2);pe(le,{code:f,get language(){return fe}}),J(2),E(H);var ue=p(H,2),Q=p(h(ue),2);pe(Q,{get code(){return r(I)},get language(){return fe}}),J(2),E(ue),i(L,re)},W=L=>{var re=X(),H=T(re);{var le=Q=>{var F=qa(),j=T(F),de=p(h(j),2);pe(de,{code:M,get language(){return fe}}),J(2),E(j);var ie=p(j,2),Z=p(h(ie),2);pe(Z,{get code(){return r(N)},get language(){return fe}}),J(2),E(ie),i(Q,F)},ue=Q=>{var F=X(),j=T(F);{var de=Z=>{var Y=Wa(),_e=T(Y),ye=p(h(_e),2);pe(ye,{get code(){return r(b)},get language(){return Ce}}),J(2),E(_e);var we=p(_e,2),Se=p(h(we),2);pe(Se,{code:y,get language(){return fe}}),J(2),E(we),i(Z,Y)},ie=Z=>{var Y=Ya(),_e=T(Y);{var ye=$e=>{var Ke=Za(),Pe=T(Ke),ht=p(h(Pe),2);pe(ht,{code:d,get language(){return fe}}),E(Pe);var Ge=p(Pe,2),He=p(h(Ge),2);ee(He,()=>Me,(St,At)=>{At(St,{get value(){return r(a)},onValueChange:xe=>{xe&&ge(a,xe,!0)},children:(xe,vr)=>{var ze=X(),Ot=T(ze);ee(Ot,()=>Ie,(Rt,xt)=>{xt(Rt,{class:"mb-2",children:(Nt,hr)=>{var qe=X(),Ct=T(qe);Oe(Ct,17,()=>Object.entries(n),([De,We])=>De,(De,We)=>{var Ze=U(()=>Bt(r(We),2));let It=()=>r(Ze)[0],Mt=()=>r(Ze)[1];var Ye=X(),Lt=T(Ye);ee(Lt,()=>Re,($t,Pt)=>{Pt($t,{get value(){return It()},children:(kt,Tr)=>{J();var Xe=be();ne(()=>oe(Xe,Mt().label)),i(kt,Xe)},$$slots:{default:!0}})}),i(De,Ye)}),i(Nt,qe)},$$slots:{default:!0}})}),i(xe,ze)},$$slots:{default:!0}})});var ke=p(He,2),Tt=h(ke,!0);E(ke);var wt=p(ke,2);pe(wt,{get code(){return n[r(a)].code},get language(){return n[r(a)].language}}),E(Ge),ne(()=>oe(Tt,n[r(a)].file)),i($e,Ke)};te(_e,$e=>{r(g)&&$e(ye)})}var we=p(_e,2),Se=h(we),Et=h(Se,!0);E(Se);var vt=p(Se,2);pe(vt,{get code(){return r(w)},get language(){return fe}}),E(we),ne(()=>oe(Et,r(g)?"Step 3: Upload after your production build":"Usage")),i(Z,Y)};te(j,Z=>{r(S)?Z(de):Z(ie,!1)},!0)}i(Q,F)};te(H,Q=>{r(v)?Q(le):Q(ue,!1)},!0)}i(L,re)};te(z,L=>{r(m)?L(q):L(W,!1)})}E(P),ne(()=>oe(u,r(c))),i(G,P)},se=G=>{var P=X(),C=T(P);{var D=u=>{var R=Va(),k=h(R);E(R),ne(()=>oe(k,`An upload token is required to upload ${r(l)??""}. Ask an organization admin to generate one
		from the Connection page.`)),i(u,R)},B=u=>{var R=rr(),k=h(R);{var z=L=>{var re=Qa();i(L,re)},q=L=>{var re=X(),H=T(re);{var le=Q=>{var F=ja();i(Q,F)},ue=Q=>{var F=X(),j=T(F);{var de=Z=>{var Y=Ja();i(Z,Y)},ie=Z=>{var Y=er(),_e=h(Y);E(Y),ne(()=>oe(_e,`Generate an upload token to start uploading ${r(l)??""} as part of your build process.`)),i(Z,Y)};te(j,Z=>{r(S)?Z(de):Z(ie,!1)},!0)}i(Q,F)};te(H,Q=>{r(v)?Q(le):Q(ue,!1)},!0)}i(L,re)};te(k,L=>{r(m)?L(z):L(q,!1)})}var W=p(k,2);Ne(W,{variant:"outline",size:"sm",onclick:A,get disabled(){return r(s)},children:(L,re)=>{var H=X(),le=T(H);{var ue=F=>{var j=tr(),de=T(j);Qt(de,{class:"mr-2 h-4 w-4"}),J(),i(F,j)},Q=F=>{var j=ar(),de=T(j);at(de,{class:"mr-2 h-4 w-4"}),J(),i(F,j)};te(le,F=>{r(s)?F(ue):F(Q,!1)})}i(L,H)},$$slots:{default:!0}}),E(R),i(u,R)};te(C,u=>{r(_)?u(D):u(B,!1)},!0)}i(G,P)};te(V,G=>{r(c)?G(ae):G(se,!1)})}var ce=p(V,2);ee(ce,()=>ra,(G,P)=>{P(G,{get open(){return r(O)},set open(C){ge(O,C,!0)},children:(C,D)=>{var B=X(),u=T(B);ee(u,()=>jt,(R,k)=>{k(R,{interactOutsideBehavior:"close",children:(z,q)=>{var W=sr(),L=T(W);ee(L,()=>Jt,(H,le)=>{le(H,{children:(ue,Q)=>{var F=nr(),j=T(F);ee(j,()=>ea,(ie,Z)=>{Z(ie,{children:(Y,_e)=>{J();var ye=be("Regenerate Upload Token");i(Y,ye)},$$slots:{default:!0}})});var de=p(j,2);ee(de,()=>ta,(ie,Z)=>{Z(ie,{children:(Y,_e)=>{J();var ye=be(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);i(Y,ye)},$$slots:{default:!0}})}),i(ue,F)},$$slots:{default:!0}})});var re=p(L,4);ee(re,()=>aa,(H,le)=>{le(H,{class:"sm:justify-between",children:(ue,Q)=>{var F=or(),j=T(F);Ne(j,{variant:"outline",onclick:()=>ge(O,!1),get disabled(){return r(s)},children:(ie,Z)=>{J();var Y=be("Cancel");i(ie,Y)},$$slots:{default:!0}});var de=p(j,2);Ne(de,{variant:"destructive",onclick:K,get disabled(){return r(s)},children:(ie,Z)=>{J();var Y=be();ne(()=>oe(Y,r(s)?"Regenerating...":"Regenerate Token")),i(ie,Y)},$$slots:{default:!0}}),i(ue,F)},$$slots:{default:!0}})}),i(z,W)},$$slots:{default:!0}})}),i(C,B)},$$slots:{default:!0}})}),i(e,$),he()}var lr=x("<!> ",1),dr=x("<!> <!>",1),ur=x("<!> <!>",1);function pr(e,t){ve(t,!0);let n=U(()=>Te.currentProject);const a=U(()=>r(n)?.framework==="flutter"),s=U(()=>r(n)?.framework==="ios"),d=U(()=>tt(Te.currentProject));var o=X(),c=T(o);{var _=m=>{Wt(m,{children:(v,S)=>{var l=ur(),g=T(l);Xt(g,{children:(f,I)=>{var M=dr(),N=T(M);Vt(N,{class:"flex items-center gap-2",children:(O,A)=>{var K=lr(),$=T(K);at($,{class:"h-5 w-5"});var V=p($);ne(()=>oe(V,` ${r(a)||r(s)?"Symbol Upload":"Source Map Upload"}`)),i(O,K)},$$slots:{default:!0}});var b=p(N,2);{var y=O=>{Yt(O,{children:(A,K)=>{J();var $=be(`Upload source maps to see original file names and line numbers in stack traces from
					minified code.`);i(A,$)},$$slots:{default:!0}})};te(b,O=>{!r(a)&&!r(s)&&O(y)})}i(f,M)},$$slots:{default:!0}});var w=p(g,2);Zt(w,{children:(f,I)=>{cr(f,{})},$$slots:{default:!0}}),i(v,l)},$$slots:{default:!0}})};te(c,m=>{r(n)&&!r(d)&&m(_)})}i(e,o),he()}var mr=x('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),gr=x('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),br=x('<p class="pt-2 text-xs text-muted-foreground"><a> </a></p>'),_r=x('<div class="p-4"><!> <!></div>'),fr=x('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),yr=x('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function rn(e,t){ve(t,!0);let n=Ae(Ve(Ta())),a=Ae(Ve(Sa()));const s={bash:fe,go:qt,javascript:Ce,typescript:ft,python:ba,gradle:Da,csharp:Ua,ruby:Ka,yaml:yt},d=U(()=>Ee.find(b=>b.id===r(n))??Ee[0]),o=U(()=>r(d).frameworks.find(b=>b.id===r(a))?.id??r(d).frameworks[0]?.id??""),c=U(()=>va(r(d).id,r(o),t.backendUrl,t.token));function _(b){const y=Ee.find(O=>O.id===b);y&&(ge(n,y.id,!0),wa(y.id))}function m(b){r(d).frameworks.some(y=>y.id===b)&&(ge(a,b,!0),Aa(b))}function v(b){return s[b??"bash"]}var S=yr(),l=T(S),g=p(h(l),2);ee(g,()=>Me,(b,y)=>{y(b,{get value(){return r(n)},onValueChange:_,children:(O,A)=>{var K=X(),$=T(K);ee($,()=>Ie,(V,ae)=>{ae(V,{class:"h-auto flex-wrap justify-start",children:(se,ce)=>{var G=X(),P=T(G);Oe(P,17,()=>Ee,C=>C.id,(C,D)=>{var B=X(),u=T(B);ee(u,()=>Re,(R,k)=>{k(R,{get value(){return r(D).id},children:(z,q)=>{J();var W=be();ne(()=>oe(W,r(D).label)),i(z,W)},$$slots:{default:!0}})}),i(C,B)}),i(se,G)},$$slots:{default:!0}})}),i(O,K)},$$slots:{default:!0}})});var w=p(g,2);{var f=b=>{var y=mr(),O=p(T(y),2);ee(O,()=>Me,(A,K)=>{K(A,{get value(){return r(o)},onValueChange:m,children:($,V)=>{var ae=X(),se=T(ae);ee(se,()=>Ie,(ce,G)=>{G(ce,{class:"h-auto flex-wrap justify-start",children:(P,C)=>{var D=X(),B=T(D);Oe(B,17,()=>r(d).frameworks,u=>u.id,(u,R)=>{var k=X(),z=T(k);ee(z,()=>Re,(q,W)=>{W(q,{get value(){return r(R).id},children:(L,re)=>{J();var H=be();ne(()=>oe(H,r(R).label)),i(L,H)},$$slots:{default:!0}})}),i(u,k)}),i(P,D)},$$slots:{default:!0}})}),i($,ae)},$$slots:{default:!0}})}),i(b,y)};te(w,b=>{r(d).frameworks.length>1&&b(f)})}E(l);var I=p(l,2);Oe(I,19,()=>r(c),b=>r(d).id+r(o)+b.title,(b,y,O)=>{var A=fr(),K=h(A),$=h(K),V=h($),ae=h(V,!0);E(V);var se=p(V,2),ce=h(se,!0);E(se),E($);var G=p($,2);{var P=B=>{var u=gr(),R=h(u,!0);E(u),ne(()=>oe(R,r(y).description)),i(B,u)};te(G,B=>{r(y).description&&B(P)})}E(K);var C=p(K,2);{var D=B=>{var u=_r(),R=h(u);{let q=U(()=>v(r(y).codeLanguage));pe(R,{get code(){return r(y).code},get language(){return r(q)}})}var k=p(R,2);{var z=q=>{var W=br(),L=h(W);Gt(L,H=>({...H,target:"_blank",rel:"noopener noreferrer",class:"underline hover:text-foreground"}),[()=>({href:Ht(r(y).link.href)})]);var re=h(L,!0);E(L),E(W),ne(()=>oe(re,r(y).link.label)),i(q,W)};te(k,q=>{r(y).link&&q(z)})}E(u),i(B,u)};te(C,B=>{r(y).code&&B(D)})}E(A),ne(()=>{oe(ae,r(O)+1),oe(ce,r(y).title)}),i(b,A)});var M=p(I,2);{var N=b=>{pr(b,{})};te(M,b=>{r(n)==="nodejs"&&b(N)})}i(e,S),he()}var Er=x('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function nn(e,t){var n=Er(),a=h(n),s=p(h(a),4);je(s,{get value(){return t.endpoint}}),E(a);var d=p(a,2),o=p(h(d),2);je(o,{get value(){return t.authHeader}}),E(d);var c=p(d,2),_=p(h(c),2);pe(_,{get code(){return t.collectorConfig},get language(){return yt}}),E(c),E(n),i(e,n)}export{an as A,pe as C,rn as O,tn as S,nn as a,jr as b,fe as c,ba as d,Xr as e,Yr as f,en as g,pr as h,Vr as i,Ce as j,Qr as k,Jr as l,Zr as p};
