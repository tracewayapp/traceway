import{i as Ot,p as it,t as ya,b as Kt}from"./BYc3jfoG.js";import{l as Ea,s as ha,p as Ta,i as k}from"./7a4wz9Fg.js";import{d as L,c as s,b as o,p as at,f,n as R,t as Ke,s as g,a as rt,e as c,g as r,h as le,r as i,k as j,i as N,l as de,u as z,x as wa,v as Dt}from"./Bq244wU7.js";import{c as ne}from"./npWQtwSL.js";import{a as yt,b as mt,T as Et}from"./UP4zC35H.js";import{e as pt,i as Sa}from"./CMOe-Whj.js";import{B as me}from"./D_PvveBf.js";import{C as Ie}from"./CLfv_bqz.js";import{C as $e}from"./v3DbPef4.js";import{a as He,s as xa}from"./BXRhph22.js";import{H as qe,g as Aa}from"./Cw-wiYxU.js";import{C as Oa,a as Ra,b as Na}from"./BobgzAwU.js";import{C as Ca}from"./DrJw9jKm.js";import{C as Ia}from"./ohiL75jn.js";import{L as $a}from"./Cvq7tg_y.js";import{A as Ma,a as La,b as ka,c as Pa,d as Da,e as Ba}from"./CNJx9-5U.js";import{t as We}from"./_rwR96iw.js";import{R as Ua}from"./Bse7pcGf.js";import{I as Fa,s as Ka}from"./B9OYPylh.js";function zt(e,t){const n=Ea(t,["children","$$slots","$$events","$$legacy"]);const a=[["path",{d:"M2.586 17.414A2 2 0 0 0 2 18.828V21a1 1 0 0 0 1 1h3a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h1a1 1 0 0 0 1-1v-1a1 1 0 0 1 1-1h.172a2 2 0 0 0 1.414-.586l.814-.814a6.5 6.5 0 1 0-4-4z"}],["circle",{cx:"16.5",cy:"7.5",r:".5",fill:"currentColor"}]];Fa(e,ha({name:"key-round"},()=>n,{get iconNode(){return a},children:(l,v)=>{var d=L(),_=s(d);Ka(_,t,"default",{},null),o(l,d)},$$slots:{default:!0}}))}const Bt="[A-Za-z$_][0-9A-Za-z$_]*",za=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],Ga=["true","false","null","undefined","NaN","Infinity"],Gt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],Ht=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],qt=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],Ha=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],qa=[].concat(qt,Gt,Ht);function Wa(e){const t=e.regex,n=(O,{after:F})=>{const J="</"+O[0].slice(1);return O.input.indexOf(J,F)!==-1},a=Bt,l={begin:"<>",end:"</>"},v=/<[A-Za-z0-9\\._:-]+\s*\/>/,d={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(O,F)=>{const J=O[0].length+O.index,ue=O.input[J];if(ue==="<"||ue===","){F.ignoreMatch();return}ue===">"&&(n(O,{after:J})||F.ignoreMatch());let ie;const _e=O.input.substring(J);if(ie=_e.match(/^\s*=/)){F.ignoreMatch();return}if((ie=_e.match(/^\s+extends\s+/))&&ie.index===0){F.ignoreMatch();return}}},_={$pattern:Bt,keyword:za,literal:Ga,built_in:qa,"variable.language":Ha},b="[0-9](_?[0-9])*",y=`\\.(${b})`,T="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",A={className:"number",variants:[{begin:`(\\b(${T})((${y})|\\.)?|(${y}))[eE][+-]?(${b})\\b`},{begin:`\\b(${T})\\b((${y})\\b|\\.)?|(${y})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},p={className:"subst",begin:"\\$\\{",end:"\\}",keywords:_,contains:[]},w={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"xml"}},E={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"css"}},m={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"graphql"}},u={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,p]},$={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},S=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,w,E,m,u,{match:/\$\d+/},A];p.contains=S.concat({begin:/\{/,end:/\}/,keywords:_,contains:["self"].concat(S)});const x=[].concat($,p.contains),C=x.concat([{begin:/(\s*)\(/,end:/\)/,keywords:_,contains:["self"].concat(x)}]),I={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:_,contains:C},P={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},B={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...Gt,...Ht]}},re={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},oe={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[I],illegal:/%/},ge={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function we(O){return t.concat("(?!",O.join("|"),")")}const ve={match:t.concat(/\b/,we([...qt,"super","import"].map(O=>`${O}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},W={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},D={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},I]},Z="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",se={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead(Z)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[I]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:_,exports:{PARAMS_CONTAINS:C,CLASS_REFERENCE:B},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),re,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,w,E,m,u,$,{match:/\$\d+/},A,B,{scope:"attr",match:a+t.lookahead(":"),relevance:0},se,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[$,e.REGEXP_MODE,{className:"function",begin:Z,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:_,contains:C}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:v},{begin:d.begin,"on:begin":d.isTrulyOpeningTag,end:d.end}],subLanguage:"xml",contains:[{begin:d.begin,end:d.end,skip:!0,contains:["self"]}]}]},oe,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[I,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},W,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[I]},ve,ge,P,D,{match:/\$[(.]/}]}}const bt={name:"javascript",register:Wa};function Za(e){const t=e.regex,n={},a={begin:/\$\{/,end:/\}/,contains:["self",{begin:/:-/,contains:[n]}]};Object.assign(n,{className:"variable",variants:[{begin:t.concat(/\$[\w\d#@][\w\d_]*/,"(?![\\w\\d])(?![$])")},a]});const l={className:"subst",begin:/\$\(/,end:/\)/,contains:[e.BACKSLASH_ESCAPE]},v=e.inherit(e.COMMENT(),{match:[/(^|\s)/,/#.*$/],scope:{2:"comment"}}),d={begin:/<<-?\s*(?=\w+)/,starts:{contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,className:"string"})]}},_={className:"string",begin:/"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,n,l]};l.contains.push(_);const b={match:/\\"/},y={className:"string",begin:/'/,end:/'/},T={match:/\\'/},A={begin:/\$?\(\(/,end:/\)\)/,contains:[{begin:/\d+#[0-9a-f]+/,className:"number"},e.NUMBER_MODE,n]},p=["fish","bash","zsh","sh","csh","ksh","tcsh","dash","scsh"],w=e.SHEBANG({binary:`(${p.join("|")})`,relevance:10}),E={className:"function",begin:/\w[\w\d_]*\s*\(\s*\)\s*\{/,returnBegin:!0,contains:[e.inherit(e.TITLE_MODE,{begin:/\w[\w\d_]*/})],relevance:0},m=["if","then","else","elif","fi","time","for","while","until","in","do","done","case","esac","coproc","function","select"],u=["true","false"],h={match:/(\/[a-z._-]+)+/},$=["break","cd","continue","eval","exec","exit","export","getopts","hash","pwd","readonly","return","shift","test","times","trap","umask","unset"],S=["alias","bind","builtin","caller","command","declare","echo","enable","help","let","local","logout","mapfile","printf","read","readarray","source","sudo","type","typeset","ulimit","unalias"],x=["autoload","bg","bindkey","bye","cap","chdir","clone","comparguments","compcall","compctl","compdescribe","compfiles","compgroups","compquote","comptags","comptry","compvalues","dirs","disable","disown","echotc","echoti","emulate","fc","fg","float","functions","getcap","getln","history","integer","jobs","kill","limit","log","noglob","popd","print","pushd","pushln","rehash","sched","setcap","setopt","stat","suspend","ttyctl","unfunction","unhash","unlimit","unsetopt","vared","wait","whence","where","which","zcompile","zformat","zftp","zle","zmodload","zparseopts","zprof","zpty","zregexparse","zsocket","zstyle","ztcp"],C=["chcon","chgrp","chown","chmod","cp","dd","df","dir","dircolors","ln","ls","mkdir","mkfifo","mknod","mktemp","mv","realpath","rm","rmdir","shred","sync","touch","truncate","vdir","b2sum","base32","base64","cat","cksum","comm","csplit","cut","expand","fmt","fold","head","join","md5sum","nl","numfmt","od","paste","ptx","pr","sha1sum","sha224sum","sha256sum","sha384sum","sha512sum","shuf","sort","split","sum","tac","tail","tr","tsort","unexpand","uniq","wc","arch","basename","chroot","date","dirname","du","echo","env","expr","factor","groups","hostid","id","link","logname","nice","nohup","nproc","pathchk","pinky","printenv","printf","pwd","readlink","runcon","seq","sleep","stat","stdbuf","stty","tee","test","timeout","tty","uname","unlink","uptime","users","who","whoami","yes"];return{name:"Bash",aliases:["sh","zsh"],keywords:{$pattern:/\b[a-z][a-z0-9._-]+\b/,keyword:m,literal:u,built_in:[...$,...S,"set","shopt",...x,...C]},contains:[w,e.SHEBANG(),E,A,v,d,h,_,b,y,T,n]}}const Ve={name:"bash",register:Za};function Ya(e){const t=e.regex,n=/(?![A-Za-z0-9])(?![$])/,a=t.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,n),l=t.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,n),v=t.concat(/[A-Z]+/,n),d={scope:"variable",match:"\\$+"+a},_={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},b={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},y=e.inherit(e.APOS_STRING_MODE,{illegal:null}),T=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(b)}),A={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(b),"on:begin":(W,D)=>{D.data._beginMatch=W[1]||W[2]},"on:end":(W,D)=>{D.data._beginMatch!==W[1]&&D.ignoreMatch()}},p=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),w=`[ 	
]`,E={scope:"string",variants:[T,y,A,p]},m={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},u=["false","null","true"],h=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],$=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],x={keyword:h,literal:(W=>{const D=[];return W.forEach(Z=>{D.push(Z),Z.toLowerCase()===Z?D.push(Z.toUpperCase()):D.push(Z.toLowerCase())}),D})(u),built_in:$},C=W=>W.map(D=>D.replace(/\|\d+$/,"")),I={variants:[{match:[/new/,t.concat(w,"+"),t.concat("(?!",C($).join("\\b|"),"\\b)"),l],scope:{1:"keyword",4:"title.class"}}]},P=t.concat(a,"\\b(?!\\()"),B={variants:[{match:[t.concat(/::/,t.lookahead(/(?!class\b)/)),P],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[l,t.concat(/::/,t.lookahead(/(?!class\b)/)),P],scope:{1:"title.class",3:"variable.constant"}},{match:[l,t.concat("::",t.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[l,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},re={scope:"attr",match:t.concat(a,t.lookahead(":"),t.lookahead(/(?!::)/))},oe={relevance:0,begin:/\(/,end:/\)/,keywords:x,contains:[re,d,B,e.C_BLOCK_COMMENT_MODE,E,m,I]},ge={relevance:0,match:[/\b/,t.concat("(?!fn\\b|function\\b|",C(h).join("\\b|"),"|",C($).join("\\b|"),"\\b)"),a,t.concat(w,"*"),t.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[oe]};oe.contains.push(ge);const we=[re,B,e.C_BLOCK_COMMENT_MODE,E,m,I],ve={begin:t.concat(/#\[\s*\\?/,t.either(l,v)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:u,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:u,keyword:["new","array"]},contains:["self",...we]},...we,{scope:"meta",variants:[{match:l},{match:v}]}]};return{case_insensitive:!1,keywords:x,contains:[ve,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},_,{scope:"variable.language",match:/\$this\b/},d,ge,B,{match:[/const/,/\s/,a],scope:{1:"keyword",3:"variable.constant"}},I,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:x,contains:["self",ve,d,B,e.C_BLOCK_COMMENT_MODE,E,m]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},E,m]}}const Wn={name:"php",register:Ya};function Xa(e){const t=e.regex,n=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),a=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],_={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:a,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},b={className:"meta",begin:/^(>>>|\.\.\.) /},y={className:"subst",begin:/\{/,end:/\}/,keywords:_,illegal:/#/},T={begin:/\{\{/,relevance:0},A={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,b],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,b],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,b,T,y]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,b,T,y]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,T,y]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,T,y]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},p="[0-9](_?[0-9])*",w=`(\\b(${p}))?\\.(${p})|\\b(${p})\\.`,E=`\\b|${a.join("|")}`,m={className:"number",relevance:0,variants:[{begin:`(\\b(${p})|(${w}))[eE][+-]?(${p})[jJ]?(?=${E})`},{begin:`(${w})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${E})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${E})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${E})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${E})`},{begin:`\\b(${p})[jJ](?=${E})`}]},u={className:"comment",begin:t.lookahead(/# type:/),end:/$/,keywords:_,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},h={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:_,contains:["self",b,m,A,e.HASH_COMMENT_MODE]}]};return y.contains=[A,m,b],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:_,illegal:/(<\/|\?)|=>/,contains:[b,m,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},A,u,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[h]},{variants:[{match:[/\bclass/,/\s+/,n,/\s*/,/\(\s*/,n,/\s*\)/]},{match:[/\bclass/,/\s+/,n]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[m,h,A]}]}}const Va={name:"python",register:Xa};function Zn(e){const t="go get go.tracewayapp.com";switch(e){case"gin":return`${t} && go get go.tracewayapp.com/tracewaygin`;case"chi":return`${t} && go get go.tracewayapp.com/tracewaychi`;case"fiber":return`${t} && go get go.tracewayapp.com/tracewayfiber`;case"fasthttp":return`${t} && go get go.tracewayapp.com/tracewayfasthttp`;case"stdlib":return`${t} && go get go.tracewayapp.com/tracewayhttp`;case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.0")';case"ios":return'.package(url: "https://github.com/tracewayapp/traceway-ios.git", from: "0.1.0")';default:return t}}function Yn(e,t,n){const a=t?`${t}@${n}/api/report`:`YOUR_TOKEN@${n}/api/report`;switch(e){case"gin":return`package main

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
}`}}function Xn(e){return e==="symfony"?`<?php
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
fatalError("Test error from Traceway integration")`:e&&Ot(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:`r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`}function Vn(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

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
}`;if(e&&Ot(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

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
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${Qa(e)}";

captureException(new Error("Test error"));`}return`r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`}function Qa(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function Qn(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android",ios:"iOS"}[e]||e}function jn(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||e==="ios"||Ot(e)?"javascript":"go"}const st=[{id:"collector",label:"Collector",frameworks:[]},{id:"nodejs",label:"Node.js",frameworks:[{id:"express",label:"Express"},{id:"nestjs",label:"NestJS"},{id:"fastify",label:"Fastify"},{id:"nextjs",label:"Next.js"},{id:"koa",label:"Koa"},{id:"other",label:"Other"}]},{id:"go",label:"Go",frameworks:[{id:"gin",label:"Gin"},{id:"echo",label:"Echo"},{id:"chi",label:"Chi"},{id:"fiber",label:"Fiber"},{id:"mux",label:"gorilla/mux"},{id:"nethttp",label:"net/http"}]},{id:"python",label:"Python",frameworks:[{id:"django",label:"Django"},{id:"flask",label:"Flask"},{id:"fastapi",label:"FastAPI"},{id:"other",label:"Other"}]},{id:"java",label:"Java",frameworks:[{id:"agent",label:"Any framework"},{id:"spring",label:"Spring Boot"}]},{id:"dotnet",label:".NET",frameworks:[]},{id:"php",label:"PHP",frameworks:[{id:"symfony",label:"Symfony"},{id:"laravel",label:"Laravel"},{id:"slim",label:"Slim"},{id:"other",label:"Other"}]},{id:"ruby",label:"Ruby",frameworks:[{id:"rails",label:"Rails"},{id:"other",label:"Other"}]},{id:"other",label:"Other",frameworks:[]}];function Wt(e,t,n=[]){return["OTEL_SERVICE_NAME=my-service",`OTEL_EXPORTER_OTLP_ENDPOINT=${e}/api/otel`,`OTEL_EXPORTER_OTLP_HEADERS=Authorization=Bearer ${t}`,...n].join(`
`)}function Fe(e,t,n=[],a=""){return{title:"Configure the Exporter",description:`Set these environment variables in your shell, .env file, or deployment config. The SDK appends /v1/traces and /v1/metrics to the endpoint automatically.${a?" "+a:""}`,code:Wt(e,t,n),codeLanguage:"bash"}}function ja(e,t){return`exporters:
  otlphttp:
    endpoint: "${e}/api/otel"
    headers:
      Authorization: "Bearer ${t}"

service:
  pipelines:
    traces:
      exporters: [otlphttp]
    metrics:
      exporters: [otlphttp]`}const Ja=`import (
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
}`,Ut={gin:{lib:"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin",snippet:`import "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

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
http.ListenAndServe(":8080", mux)`,note:"Wrap each route individually with Go 1.22+ method patterns so the route is set on spans and endpoints group by pattern instead of raw URL."}},er="npm install @opentelemetry/api @opentelemetry/auto-instrumentations-node";function At(e,t,n,a,l){return[{title:"Install the SDK",description:a,code:er,codeLanguage:"bash"},Fe(e,t),{title:"Run with Instrumentation",description:l,code:`node --require @opentelemetry/auto-instrumentations-node/register ${n}`,codeLanguage:"bash"}]}function tr(e,t,n,a){switch(e){case"collector":return[{title:"Add the Traceway Exporter",description:"Merge this into your OpenTelemetry Collector configuration. Any pipeline that lists the otlphttp exporter will be forwarded to Traceway.",code:ja(n,a),codeLanguage:"yaml"},{title:"Restart the Collector",description:"Restart the Collector to apply the configuration. Traces and metrics flowing through its pipelines will appear in Traceway."}];case"nodejs":return t==="fastify"?[{title:"Install the SDK",description:"Fastify is instrumented by the @fastify/otel package maintained by the Fastify team.",code:"npm install @opentelemetry/api @opentelemetry/sdk-node @opentelemetry/auto-instrumentations-node @fastify/otel",codeLanguage:"bash"},{title:"Create instrumentation.js",description:"Add this file at the project root.",code:`const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { FastifyOtelInstrumentation } = require('@fastify/otel');

new NodeSDK({
  instrumentations: [
    getNodeAutoInstrumentations(),
    new FastifyOtelInstrumentation({ registerOnInitialization: true }),
  ],
}).start();`,codeLanguage:"javascript"},Fe(n,a),{title:"Run with Instrumentation",code:"node --require ./instrumentation.js app.js",codeLanguage:"bash"}]:t==="nextjs"?[{title:"Install the SDK",code:"npm install @vercel/otel",codeLanguage:"bash"},{title:"Create instrumentation.ts",description:"Add this file at the project root (next to package.json). Next.js calls register() automatically on startup.",code:`import { registerOTel } from '@vercel/otel'

export function register() {
  registerOTel({ serviceName: 'my-service' })
}`,codeLanguage:"typescript"},Fe(n,a,[],"Start your app normally with next start; no extra flags are needed.")]:t==="nestjs"?At(n,a,"dist/main.js","Auto-instrumentation captures NestJS routes, status codes, and errors through the default Express adapter with no code changes. If you use the Fastify adapter, follow the Fastify setup instead.","Routes group by pattern automatically."):t==="koa"?At(n,a,"app.js","Auto-instrumentation captures Koa requests, status codes, and errors with no code changes.","Route patterns are captured when routing with @koa/router."):At(n,a,"app.js","Auto-instrumentation captures routes, status codes, and errors with no code changes.","For ESM apps, add --experimental-loader=@opentelemetry/instrumentation/hook.mjs and use --import instead of --require.");case"go":{const l=Ut[t]??Ut.gin;return[{title:"Install the SDK",code:`go get go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp ${l.lib}`,codeLanguage:"bash"},{title:"Initialize the SDK",description:"Call initTracer at startup and defer tp.Shutdown(ctx) before exit. The exporter reads the environment variables from the next step.",code:Ja,codeLanguage:"go"},{title:"Add the Middleware",description:l.note,code:l.snippet,codeLanguage:"go"},Fe(n,a)]}case"python":{const l={django:{cmd:"opentelemetry-instrument python manage.py runserver --noreload",note:"The --noreload flag is required with runserver; the autoreloader breaks instrumentation. It is not needed under gunicorn or other production servers."},flask:{cmd:"opentelemetry-instrument flask run"},fastapi:{cmd:"opentelemetry-instrument uvicorn main:app",note:"Avoid --reload and --workers with zero-code instrumentation; for multi-worker production use gunicorn with uvicorn workers."},other:{cmd:"opentelemetry-instrument python app.py"}},v=l[t]??l.other;return[{title:"Install the SDK",description:"opentelemetry-bootstrap detects your installed packages and adds the matching instrumentation.",code:`pip install opentelemetry-distro opentelemetry-exporter-otlp-proto-http
opentelemetry-bootstrap -a install`,codeLanguage:"bash"},Fe(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the Python SDK defaults to gRPC."),{title:"Run with Instrumentation",description:v.note,code:v.cmd,codeLanguage:"bash"}]}case"java":return t==="spring"?[{title:"Add the Starter",description:"Add the OpenTelemetry Spring Boot starter to your Gradle build (a Maven dependency works the same way).",code:`implementation(platform("io.opentelemetry.instrumentation:opentelemetry-instrumentation-bom:2.28.1"))
implementation("io.opentelemetry.instrumentation:opentelemetry-spring-boot-starter")`,codeLanguage:"gradle"},Fe(n,a,[],"Start your app normally; the starter reads these variables and reports routes, status codes, and exceptions.")]:[{title:"Download the Java Agent",description:"The agent instruments Spring, JAX-RS, and most Java frameworks with zero code changes.",code:"curl -L -O https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/latest/download/opentelemetry-javaagent.jar",codeLanguage:"bash"},Fe(n,a),{title:"Run with the Agent",code:"java -javaagent:./opentelemetry-javaagent.jar -jar myapp.jar",codeLanguage:"bash"}];case"dotnet":return[{title:"Install the Packages",code:`dotnet add package OpenTelemetry.Extensions.Hosting
dotnet add package OpenTelemetry.Instrumentation.AspNetCore
dotnet add package OpenTelemetry.Exporter.OpenTelemetryProtocol`,codeLanguage:"bash"},{title:"Add to Program.cs",description:"Keep AddOtlpExporter() empty so the exporter is driven entirely by the environment variables in the next step.",code:`builder.Services.AddOpenTelemetry()
    .WithTracing(t => t
        .AddAspNetCoreInstrumentation()
        .AddOtlpExporter());`,codeLanguage:"csharp"},Fe(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"],"The protocol variable is required; the .NET exporter defaults to gRPC.")];case"php":{const v={symfony:" open-telemetry/opentelemetry-auto-symfony",laravel:" open-telemetry/opentelemetry-auto-laravel",slim:" open-telemetry/opentelemetry-auto-slim",other:""}[t]??"";return[{title:"Install the SDK",description:"Auto-instrumentation needs the opentelemetry PECL extension; enable it with extension=opentelemetry in php.ini."+(t==="other"?" Find auto-instrumentation packages for your framework in the OpenTelemetry registry.":""),code:`pecl install opentelemetry
composer require open-telemetry/sdk open-telemetry/exporter-otlp php-http/guzzle7-adapter${v}`,codeLanguage:"bash",link:t==="other"?{label:"Browse PHP instrumentation packages",href:"https://opentelemetry.io/ecosystem/registry/?language=php&component=instrumentation"}:void 0},Fe(n,a,["OTEL_PHP_AUTOLOAD_ENABLED=true"],"These must be real process environment variables; the extension does not read framework .env files. Use env[...] in php-fpm pool config or SetEnv in Apache.")]}case"ruby":return t==="rails"?[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-rails",codeLanguage:"bash"},{title:"Create the Initializer",description:"Add config/initializers/opentelemetry.rb.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/rails'

OpenTelemetry::SDK.configure do |c|
  c.use 'OpenTelemetry::Instrumentation::Rails'
end`,codeLanguage:"ruby"},Fe(n,a)]:[{title:"Install the Gems",code:"bundle add opentelemetry-sdk opentelemetry-exporter-otlp opentelemetry-instrumentation-all",codeLanguage:"bash"},{title:"Configure the SDK",description:"Run this once at startup, before your app starts handling requests.",code:`require 'opentelemetry/sdk'
require 'opentelemetry/exporter/otlp'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure do |c|
  c.use_all
end`,codeLanguage:"ruby"},Fe(n,a)];default:return[{title:"Configure any OpenTelemetry SDK",description:"Any language with an OTLP/HTTP exporter works. Set these environment variables; the protocol variable matters for SDKs that default to gRPC. Make sure http.route is set on root server spans so endpoints group by route pattern, and use SpanKind CONSUMER for background jobs.",code:Wt(n,a,["OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf"]),codeLanguage:"bash",link:{label:"View all supported languages",href:"https://opentelemetry.io/docs/languages/"}}]}}const Zt="traceway_setup_mode",Yt="traceway_otel_language",Xt="traceway_otel_framework";function Jn(){try{const e=localStorage.getItem(Zt);if(e==="ai"||e==="manual")return e}catch{}return"ai"}function ar(e){try{localStorage.setItem(Zt,e)}catch{}}function rr(){try{const e=localStorage.getItem(Yt);if(e&&st.some(t=>t.id===e))return e}catch{}return st[0].id}function nr(e){try{localStorage.setItem(Yt,e)}catch{}}function or(){try{return localStorage.getItem(Xt)}catch{}return null}function sr(e){try{localStorage.setItem(Xt,e)}catch{}}var ir=f("<!> <!>",1);function eo(e,t){at(t,!0);const n="-mb-px rounded-none border-b-2 border-transparent bg-transparent px-0 pb-2.5 pt-0 text-sm font-medium text-muted-foreground shadow-none data-[state=active]:border-primary data-[state=active]:bg-transparent data-[state=active]:text-foreground data-[state=active]:shadow-none";function a(d){(d==="ai"||d==="manual")&&(ar(d),t.onModeChange(d))}var l=L(),v=s(l);ne(v,()=>Et,(d,_)=>{_(d,{get value(){return t.mode},onValueChange:a,children:(b,y)=>{var T=L(),A=s(T);ne(A,()=>yt,(p,w)=>{w(p,{class:"h-auto w-full justify-start gap-4 rounded-none border-b bg-transparent p-0",children:(E,m)=>{var u=ir(),h=s(u);ne(h,()=>mt,(S,x)=>{x(S,{value:"ai",class:n,children:(C,I)=>{R();var P=Ke("AI");o(C,P)},$$slots:{default:!0}})});var $=g(h,2);ne($,()=>mt,(S,x)=>{x(S,{value:"manual",class:n,children:(C,I)=>{R();var P=Ke("Manual");o(C,P)},$$slots:{default:!0}})}),o(E,u)},$$slots:{default:!0}})}),o(b,T)},$$slots:{default:!0}})}),o(e,l),rt()}const cr="npx skills add tracewayapp/traceway";function lr(e,t,n=null){const a=[{text:"/traceway-setup with token ",bold:!1},{text:t,bold:!0},{text:" and url ",bold:!1},{text:e,bold:!0}];return n&&a.push({text:" and source map upload token ",bold:!1},{text:n,bold:!0}),a}var dr=f("<!> Copied!",1),ur=f("<!> Copy",1),pr=f('<div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div>');function Rt(e,t){at(t,!0);let n=Ta(t,"wrap",3,!1),a=le(!1);async function l(){await navigator.clipboard.writeText(t.code),N(a,!0),setTimeout(()=>N(a,!1),2e3)}var v=pr(),d=c(v),_=c(d);me(_,{variant:"outline",size:"sm",onclick:l,children:(T,A)=>{var p=L(),w=s(p);{var E=u=>{var h=dr(),$=s(h);Ie($,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(u,h)},m=u=>{var h=ur(),$=s(h);$e($,{class:"mr-2 h-4 w-4"}),R(),o(u,h)};k(w,u=>{r(a)?u(E):u(m,!1)})}o(T,p)},$$slots:{default:!0}}),i(d);var b=g(d,2),y=c(b);qe(y,{get language(){return t.language},get code(){return t.code}}),i(b),i(v),j(()=>He(b,1,`overflow-x-auto rounded-lg text-sm ${n()?"wrap-code":""} ${We.isDark?"dark-code":"light-code"}`)),o(e,v),rt()}var mr=f("<!> Copied!",1),gr=f("<!> Copy",1),vr=f('<span class="break-all text-muted-foreground"> </span>'),_r=f('<div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <code class="block rounded-lg bg-muted py-3 pr-24 pl-4 font-mono text-sm break-words whitespace-pre-wrap text-foreground"></code></div>');function fr(e,t){at(t,!0);const n=z(()=>t.parts.map(y=>y.text).join(""));let a=le(!1);async function l(){await navigator.clipboard.writeText(r(n)),N(a,!0),setTimeout(()=>N(a,!1),2e3)}var v=_r(),d=c(v),_=c(d);me(_,{variant:"outline",size:"sm",onclick:l,children:(y,T)=>{var A=L(),p=s(A);{var w=m=>{var u=mr(),h=s(u);Ie(h,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(m,u)},E=m=>{var u=gr(),h=s(u);$e(h,{class:"mr-2 h-4 w-4"}),R(),o(m,u)};k(p,m=>{r(a)?m(w):m(E,!1)})}o(y,A)},$$slots:{default:!0}}),i(d);var b=g(d,2);pt(b,21,()=>t.parts,Sa,(y,T)=>{var A=L(),p=s(A);{var w=m=>{var u=vr(),h=c(u,!0);i(u),j(()=>de(h,r(T).text)),o(m,u)},E=m=>{var u=Ke();j(()=>de(u,r(T).text)),o(m,u)};k(p,m=>{r(T).bold?m(w):m(E,!1)})}o(y,A)}),i(b),i(v),o(e,v),rt()}var br=f(`<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">1</div> <h3 class="font-semibold">Install the Traceway Skill</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground">Add the Traceway setup skill to your coding agent. Works with Claude Code, Cursor, and any
			agent that supports agent skills.</p></div> <div class="p-4"><!></div></div> <div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground">2</div> <h3 class="font-semibold">Run the Setup Prompt</h3></div> <p class="mt-1 ml-9 text-sm text-muted-foreground"> </p></div> <div class="p-4"><!></div></div>`,1);function to(e,t){at(t,!0);const n=z(()=>it.currentProject?.sourceMapToken??null),a=z(()=>lr(t.backendUrl,t.token,r(n)));var l=br(),v=s(l),d=g(c(v),2),_=c(d);Rt(_,{get code(){return cr},get language(){return Ve}}),i(d),i(v);var b=g(v,2),y=c(b),T=g(c(y),2),A=c(T);i(T),i(y);var p=g(y,2),w=c(p);fr(w,{get parts(){return r(a)}}),i(p),i(b),j(()=>de(A,`Paste this prompt into your agent. Your instance URL and project token are already filled
			in${r(n)?", along with your source map upload token":""}.`)),o(e,l),rt()}const ht="[A-Za-z$_][0-9A-Za-z$_]*",Vt=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],Qt=["true","false","null","undefined","NaN","Infinity"],jt=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],Jt=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],ea=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],ta=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],aa=[].concat(ea,jt,Jt);function yr(e){const t=e.regex,n=(O,{after:F})=>{const J="</"+O[0].slice(1);return O.input.indexOf(J,F)!==-1},a=ht,l={begin:"<>",end:"</>"},v=/<[A-Za-z0-9\\._:-]+\s*\/>/,d={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(O,F)=>{const J=O[0].length+O.index,ue=O.input[J];if(ue==="<"||ue===","){F.ignoreMatch();return}ue===">"&&(n(O,{after:J})||F.ignoreMatch());let ie;const _e=O.input.substring(J);if(ie=_e.match(/^\s*=/)){F.ignoreMatch();return}if((ie=_e.match(/^\s+extends\s+/))&&ie.index===0){F.ignoreMatch();return}}},_={$pattern:ht,keyword:Vt,literal:Qt,built_in:aa,"variable.language":ta},b="[0-9](_?[0-9])*",y=`\\.(${b})`,T="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",A={className:"number",variants:[{begin:`(\\b(${T})((${y})|\\.)?|(${y}))[eE][+-]?(${b})\\b`},{begin:`\\b(${T})\\b((${y})\\b|\\.)?|(${y})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},p={className:"subst",begin:"\\$\\{",end:"\\}",keywords:_,contains:[]},w={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"xml"}},E={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"css"}},m={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,p],subLanguage:"graphql"}},u={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,p]},$={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},S=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,w,E,m,u,{match:/\$\d+/},A];p.contains=S.concat({begin:/\{/,end:/\}/,keywords:_,contains:["self"].concat(S)});const x=[].concat($,p.contains),C=x.concat([{begin:/(\s*)\(/,end:/\)/,keywords:_,contains:["self"].concat(x)}]),I={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:_,contains:C},P={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},B={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...jt,...Jt]}},re={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},oe={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[I],illegal:/%/},ge={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function we(O){return t.concat("(?!",O.join("|"),")")}const ve={match:t.concat(/\b/,we([...ea,"super","import"].map(O=>`${O}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},W={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},D={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},I]},Z="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",se={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead(Z)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[I]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:_,exports:{PARAMS_CONTAINS:C,CLASS_REFERENCE:B},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),re,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,w,E,m,u,$,{match:/\$\d+/},A,B,{scope:"attr",match:a+t.lookahead(":"),relevance:0},se,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[$,e.REGEXP_MODE,{className:"function",begin:Z,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:_,contains:C}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:l.begin,end:l.end},{match:v},{begin:d.begin,"on:begin":d.isTrulyOpeningTag,end:d.end}],subLanguage:"xml",contains:[{begin:d.begin,end:d.end,skip:!0,contains:["self"]}]}]},oe,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[I,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},W,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[I]},ve,ge,P,D,{match:/\$[(.]/}]}}function Er(e){const t=e.regex,n=yr(e),a=ht,l=["any","void","number","boolean","string","object","never","symbol","bigint","unknown"],v={begin:[/namespace/,/\s+/,e.IDENT_RE],beginScope:{1:"keyword",3:"title.class"}},d={beginKeywords:"interface",end:/\{/,excludeEnd:!0,keywords:{keyword:"interface extends",built_in:l},contains:[n.exports.CLASS_REFERENCE]},_={className:"meta",relevance:10,begin:/^\s*['"]use strict['"]/},b=["type","interface","public","private","protected","implements","declare","abstract","readonly","enum","override","satisfies"],y={$pattern:ht,keyword:Vt.concat(b),literal:Qt,built_in:aa.concat(l),"variable.language":ta},T={className:"meta",begin:"@"+a},A=(m,u,h)=>{const $=m.contains.findIndex(S=>S.label===u);if($===-1)throw new Error("can not find mode to replace");m.contains.splice($,1,h)};Object.assign(n.keywords,y),n.exports.PARAMS_CONTAINS.push(T);const p=n.contains.find(m=>m.scope==="attr"),w=Object.assign({},p,{match:t.concat(a,t.lookahead(/\s*\?:/))});n.exports.PARAMS_CONTAINS.push([n.exports.CLASS_REFERENCE,p,w]),n.contains=n.contains.concat([T,v,d,w]),A(n,"shebang",e.SHEBANG()),A(n,"use_strict",_);const E=n.contains.find(m=>m.label==="func.def");return E.relevance=0,Object.assign(n,{name:"TypeScript",aliases:["ts","tsx","mts","cts"]}),n}const ra={name:"typescript",register:Er};function hr(e){return{name:"Gradle",case_insensitive:!0,keywords:["task","project","allprojects","subprojects","artifacts","buildscript","configurations","dependencies","repositories","sourceSets","description","delete","from","into","include","exclude","source","classpath","destinationDir","includes","options","sourceCompatibility","targetCompatibility","group","flatDir","doLast","doFirst","flatten","todir","fromdir","ant","def","abstract","break","case","catch","continue","default","do","else","extends","final","finally","for","if","implements","instanceof","native","new","private","protected","public","return","static","switch","synchronized","throw","throws","transient","try","volatile","while","strictfp","package","import","false","null","super","this","true","antlrtask","checkstyle","codenarc","copy","boolean","byte","char","class","double","float","int","interface","long","short","void","compile","runTime","file","fileTree","abs","any","append","asList","asWritable","call","collect","compareTo","count","div","dump","each","eachByte","eachFile","eachLine","every","find","findAll","flatten","getAt","getErr","getIn","getOut","getText","grep","immutable","inject","inspect","intersect","invokeMethods","isCase","join","leftShift","minus","multiply","newInputStream","newOutputStream","newPrintWriter","newReader","newWriter","next","plus","pop","power","previous","print","println","push","putAt","read","readBytes","readLines","reverse","reverseEach","round","size","sort","splitEachLine","step","subMap","times","toInteger","toList","tokenize","upto","waitForOrKill","withPrintWriter","withReader","withStream","withWriter","withWriterAppend","write","writeLine"],contains:[e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,e.NUMBER_MODE,e.REGEXP_MODE]}}const Tr={name:"gradle",register:hr};function wr(e){const t=["bool","byte","char","decimal","delegate","double","dynamic","enum","float","int","long","nint","nuint","object","sbyte","short","string","ulong","uint","ushort"],n=["public","private","protected","static","internal","protected","abstract","async","extern","override","unsafe","virtual","new","sealed","partial"],a=["default","false","null","true"],l=["abstract","as","base","break","case","catch","class","const","continue","do","else","event","explicit","extern","finally","fixed","for","foreach","goto","if","implicit","in","interface","internal","is","lock","namespace","new","operator","out","override","params","private","protected","public","readonly","record","ref","return","scoped","sealed","sizeof","stackalloc","static","struct","switch","this","throw","try","typeof","unchecked","unsafe","using","virtual","void","volatile","while"],v=["add","alias","and","ascending","args","async","await","by","descending","dynamic","equals","file","from","get","global","group","init","into","join","let","nameof","not","notnull","on","or","orderby","partial","record","remove","required","scoped","select","set","unmanaged","value|0","var","when","where","with","yield"],d={keyword:l.concat(v),built_in:t,literal:a},_=e.inherit(e.TITLE_MODE,{begin:"[a-zA-Z](\\.?\\w)*"}),b={className:"number",variants:[{begin:"\\b(0b[01']+)"},{begin:"(-?)\\b([\\d']+(\\.[\\d']*)?|\\.[\\d']+)(u|U|l|L|ul|UL|f|F|b|B)"},{begin:"(-?)(\\b0[xX][a-fA-F0-9']+|(\\b[\\d']+(\\.[\\d']*)?|\\.[\\d']+)([eE][-+]?[\\d']+)?)"}],relevance:0},y={className:"string",begin:/"""("*)(?!")(.|\n)*?"""\1/,relevance:1},T={className:"string",begin:'@"',end:'"',contains:[{begin:'""'}]},A=e.inherit(T,{illegal:/\n/}),p={className:"subst",begin:/\{/,end:/\}/,keywords:d},w=e.inherit(p,{illegal:/\n/}),E={className:"string",begin:/\$"/,end:'"',illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},e.BACKSLASH_ESCAPE,w]},m={className:"string",begin:/\$@"/,end:'"',contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},p]},u=e.inherit(m,{illegal:/\n/,contains:[{begin:/\{\{/},{begin:/\}\}/},{begin:'""'},w]});p.contains=[m,E,T,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,e.C_BLOCK_COMMENT_MODE],w.contains=[u,E,A,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,b,e.inherit(e.C_BLOCK_COMMENT_MODE,{illegal:/\n/})];const h={variants:[y,m,E,T,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},$={begin:"<",end:">",contains:[{beginKeywords:"in out"},_]},S=e.IDENT_RE+"(<"+e.IDENT_RE+"(\\s*,\\s*"+e.IDENT_RE+")*>)?(\\[\\])?",x={begin:"@"+e.IDENT_RE,relevance:0};return{name:"C#",aliases:["cs","c#"],keywords:d,illegal:/::/,contains:[e.COMMENT("///","$",{returnBegin:!0,contains:[{className:"doctag",variants:[{begin:"///",relevance:0},{begin:"<!--|-->"},{begin:"</?",end:">"}]}]}),e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE,{className:"meta",begin:"#",end:"$",keywords:{keyword:"if else elif endif define undef warning error line region endregion pragma checksum"}},h,b,{beginKeywords:"class interface",relevance:0,end:/[{;=]/,illegal:/[^\s:,]/,contains:[{beginKeywords:"where class"},_,$,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"namespace",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[_,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{beginKeywords:"record",relevance:0,end:/[{;=]/,illegal:/[^\s:]/,contains:[_,$,e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},{className:"meta",begin:"^\\s*\\[(?=[\\w])",excludeBegin:!0,end:"\\]",excludeEnd:!0,contains:[{className:"string",begin:/"/,end:/"/}]},{beginKeywords:"new return throw await else",relevance:0},{className:"function",begin:"("+S+"\\s+)+"+e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,end:/\s*[{;=]/,excludeEnd:!0,keywords:d,contains:[{beginKeywords:n.join(" "),relevance:0},{begin:e.IDENT_RE+"\\s*(<[^=]+>\\s*)?\\(",returnBegin:!0,contains:[e.TITLE_MODE,$],relevance:0},{match:/\(\)/},{className:"params",begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:d,relevance:0,contains:[h,b,e.C_BLOCK_COMMENT_MODE]},e.C_LINE_COMMENT_MODE,e.C_BLOCK_COMMENT_MODE]},x]}}const Sr={name:"csharp",register:wr};function xr(e){const t=e.regex,n="([a-zA-Z_]\\w*[!?=]?|[-+~]@|<<|>>|=~|===?|<=>|[<>]=?|\\*\\*|[-/+%^&*~`|]|\\[\\]=?)",a=t.either(/\b([A-Z]+[a-z0-9]+)+/,/\b([A-Z]+[a-z0-9]+)+[A-Z]+/),l=t.concat(a,/(::\w+)*/),d={"variable.constant":["__FILE__","__LINE__","__ENCODING__"],"variable.language":["self","super"],keyword:["alias","and","begin","BEGIN","break","case","class","defined","do","else","elsif","end","END","ensure","for","if","in","module","next","not","or","redo","require","rescue","retry","return","then","undef","unless","until","when","while","yield",...["include","extend","prepend","public","private","protected","raise","throw"]],built_in:["proc","lambda","attr_accessor","attr_reader","attr_writer","define_method","private_constant","module_function"],literal:["true","false","nil"]},_={className:"doctag",begin:"@[A-Za-z]+"},b={begin:"#<",end:">"},y=[e.COMMENT("#","$",{contains:[_]}),e.COMMENT("^=begin","^=end",{contains:[_],relevance:10}),e.COMMENT("^__END__",e.MATCH_NOTHING_RE)],T={className:"subst",begin:/#\{/,end:/\}/,keywords:d},A={className:"string",contains:[e.BACKSLASH_ESCAPE,T],variants:[{begin:/'/,end:/'/},{begin:/"/,end:/"/},{begin:/`/,end:/`/},{begin:/%[qQwWx]?\(/,end:/\)/},{begin:/%[qQwWx]?\[/,end:/\]/},{begin:/%[qQwWx]?\{/,end:/\}/},{begin:/%[qQwWx]?</,end:/>/},{begin:/%[qQwWx]?\//,end:/\//},{begin:/%[qQwWx]?%/,end:/%/},{begin:/%[qQwWx]?-/,end:/-/},{begin:/%[qQwWx]?\|/,end:/\|/},{begin:/\B\?(\\\d{1,3})/},{begin:/\B\?(\\x[A-Fa-f0-9]{1,2})/},{begin:/\B\?(\\u\{?[A-Fa-f0-9]{1,6}\}?)/},{begin:/\B\?(\\M-\\C-|\\M-\\c|\\c\\M-|\\M-|\\C-\\M-)[\x20-\x7e]/},{begin:/\B\?\\(c|C-)[\x20-\x7e]/},{begin:/\B\?\\?\S/},{begin:t.concat(/<<[-~]?'?/,t.lookahead(/(\w+)(?=\W)[^\n]*\n(?:[^\n]*\n)*?\s*\1\b/)),contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,contains:[e.BACKSLASH_ESCAPE,T]})]}]},p="[1-9](_?[0-9])*|0",w="[0-9](_?[0-9])*",E={className:"number",relevance:0,variants:[{begin:`\\b(${p})(\\.(${w}))?([eE][+-]?(${w})|r)?i?\\b`},{begin:"\\b0[dD][0-9](_?[0-9])*r?i?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*r?i?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*r?i?\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*r?i?\\b"},{begin:"\\b0(_?[0-7])+r?i?\\b"}]},m={variants:[{match:/\(\)/},{className:"params",begin:/\(/,end:/(?=\))/,excludeBegin:!0,endsParent:!0,keywords:d}]},I=[A,{variants:[{match:[/class\s+/,l,/\s+<\s+/,l]},{match:[/\b(class|module)\s+/,l]}],scope:{2:"title.class",4:"title.class.inherited"},keywords:d},{match:[/(include|extend)\s+/,l],scope:{2:"title.class"},keywords:d},{relevance:0,match:[l,/\.new[. (]/],scope:{1:"title.class"}},{relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"},{relevance:0,match:a,scope:"title.class"},{match:[/def/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[m]},{begin:e.IDENT_RE+"::"},{className:"symbol",begin:e.UNDERSCORE_IDENT_RE+"(!|\\?)?:",relevance:0},{className:"symbol",begin:":(?!\\s)",contains:[A,{begin:n}],relevance:0},E,{className:"variable",begin:"(\\$\\W)|((\\$|@@?)(\\w+))(?=[^@$?])(?![A-Za-z])(?![@$?'])"},{className:"params",begin:/\|(?!=)/,end:/\|/,excludeBegin:!0,excludeEnd:!0,relevance:0,keywords:d},{begin:"("+e.RE_STARTERS_RE+"|unless)\\s*",keywords:"unless",contains:[{className:"regexp",contains:[e.BACKSLASH_ESCAPE,T],illegal:/\n/,variants:[{begin:"/",end:"/[a-z]*"},{begin:/%r\{/,end:/\}[a-z]*/},{begin:"%r\\(",end:"\\)[a-z]*"},{begin:"%r!",end:"![a-z]*"},{begin:"%r\\[",end:"\\][a-z]*"}]}].concat(b,y),relevance:0}].concat(b,y);T.contains=I,m.contains=I;const oe=[{begin:/^\s*=>/,starts:{end:"$",contains:I}},{className:"meta.prompt",begin:"^("+"[>?]>"+"|"+"[\\w#]+\\(\\w+\\):\\d+:\\d+[>*]"+"|"+"(\\w+-)?\\d+\\.\\d+\\.\\d+(p\\d+)?[^\\d][^>]+>"+")(?=[ ])",starts:{end:"$",keywords:d,contains:I}}];return y.unshift(b),{name:"Ruby",aliases:["rb","gemspec","podspec","thor","irb"],keywords:d,illegal:/\/\*/,contains:[e.SHEBANG({binary:"ruby"})].concat(oe).concat(y).concat(I)}}const Ar={name:"ruby",register:xr};function Or(e){const t="true false yes no null",n="[\\w#;/?:@&=+$,.~*'()[\\]]+",a={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},l={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},v={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},d={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,l]},_=e.inherit(d,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),p={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},w={end:",",endsWithParent:!0,excludeEnd:!0,keywords:t,relevance:0},E={begin:/\{/,end:/\}/,contains:[w],illegal:"\\n",relevance:0},m={begin:"\\[",end:"\\]",contains:[w],illegal:"\\n",relevance:0},u=[a,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+n},{className:"type",begin:"!<"+n+">"},{className:"type",begin:"!"+n},{className:"type",begin:"!!"+n},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:t,keywords:{literal:t}},p,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},E,m,v,d],h=[...u];return h.pop(),h.push(_),w.contains=h,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:u}}const na={name:"yaml",register:Or};var Rr=f("<!> Regenerate",1),Nr=f("<!> Copied!",1),Cr=f("<!> Copy",1),Ir=f("<!> Copied!",1),$r=f("<!> Copy",1),Mr=f(`<div><p class="mb-2 text-sm font-medium">Step 1: Build with obfuscation enabled</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">This writes a per-architecture .symbols file into build/symbols. The example builds an
					Android APK; other targets emit their own symbol files in the same directory.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the symbols after each release build</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">Run from your project root after each release. The uploader auto-discovers build/symbols
					and pushes every architecture in one go; symbols are unique per build, so re-upload on
					each release. In CI, pass the token as <code class="font-mono">TRACEWAY_UPLOAD_TOKEN</code> instead of the flag.</p></div>`,1),Lr=f("<!> Copied!",1),kr=f("<!> Copy",1),Pr=f("<!> Copied!",1),Dr=f("<!> Copy",1),Br=f(`<div><p class="mb-2 text-sm font-medium">Step 1: Build an archive with dSYMs</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">Release builds emit a .dSYM bundle per architecture under the archive's dSYMs directory.
					Replace MyApp with your scheme name.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Upload the dSYM after each release build</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">Upload the Mach-O DWARF inside the .dSYM bundle. Symbols are keyed by build UUID, so
					re-upload on each release.</p></div>`,1),Ur=f("<!> Copied!",1),Fr=f("<!> Copy",1),Kr=f("<!> Copied!",1),zr=f("<!> Copy",1),Gr=f(`<div><p class="mb-2 text-sm font-medium">Step 1: Apply the Traceway symbols Gradle plugin</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">Add to your app module's <code class="font-mono">build.gradle.kts</code>. The plugin embeds a
					ProGuard UUID into BuildConfig (matching Honeycomb's <code class="font-mono">app.debug.proguard_uuid</code>) and names the uploaded mapping <code class="font-mono">&lt;uuid&gt;.txt</code>.</p></div> <div><p class="mb-2 text-sm font-medium">Step 2: Build and upload after each release</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div> <p class="mt-2 text-xs text-muted-foreground">Uploads the R8 <code class="font-mono">mapping.txt</code> and the unstripped native <code class="font-mono">.so</code> libraries. Native symbols are keyed by GNU build-id, so
					re-upload on each release.</p></div>`,1),Hr=f("<!> Copied!",1),qr=f("<!> Copy",1),Wr=f("<!> Copied!",1),Zr=f("<!> Copy",1),Yr=f('<div><p class="mb-2 text-sm font-medium">Step 1: Install the bundler plugin</p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div> <div><p class="mb-2 text-sm font-medium">Step 2: Add the plugin to your bundler</p> <!> <p class="mb-2 font-mono text-xs text-muted-foreground"> </p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div>',1),Xr=f("<!> Copied!",1),Vr=f("<!> Copy",1),Qr=f('<!> <div><p class="mb-2 text-sm font-medium"> </p> <div class="relative"><div class="absolute top-2 right-2 z-10"><!></div> <div><!></div></div></div>',1),jr=f('<div class="space-y-6"><div><p class="mb-2 text-sm font-medium">Upload Token</p> <div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!> <!></div></div> <!></div>'),Jr=f('<p class="text-sm text-muted-foreground"> </p>'),en=f('<p class="text-sm text-muted-foreground">Plain release builds already report readable traces. Only obfuscated builds (<code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">--obfuscate</code>) need this: generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.symbols</code> after each release to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/flutter" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Flutter docs</a></p>'),tn=f(`<p class="text-sm text-muted-foreground">Release crashes report against stripped machine code. Generate a token, then upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.dSYM</code> after each release
				to resolve their stack traces. <a href="https://docs.tracewayapp.com/client/ios" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">iOS docs</a></p>`),an=f(`<p class="text-sm text-muted-foreground">Release builds obfuscate Kotlin/Java with R8 and strip native code. Generate a token, then
				upload your <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">mapping.txt</code> and
				native <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">.so</code> libraries after each
				release to resolve their stack traces. <a href="https://docs.tracewayapp.com/symbolicator/android" target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground">Android docs</a></p>`),rn=f('<p class="text-sm text-muted-foreground"> </p>'),nn=f("<!> Generating...",1),on=f("<!> Generate Upload Token",1),sn=f('<div class="flex items-center justify-between gap-4"><!> <!></div>'),cn=f("<!> <!>",1),ln=f("<!> <!>",1),dn=f(`<!> <div class="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2"><p class="text-sm"><span class="font-semibold text-destructive">Warning:</span> <span class="text-destructive/90">Any build pipeline or CI job still using the current token will fail to upload source
					maps until it is updated with the new token.</span></p></div> <!>`,1),un=f("<!> <!>",1);function pn(e,t){at(t,!0);const n={vite:{label:"Vite",file:"vite.config.ts",directory:"dist/assets",language:ra,code:`import { defineConfig } from "vite";
import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/vite";

export default defineConfig({
  build: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
});`},rollup:{label:"Rollup",file:"rollup.config.js",directory:"dist",language:bt,code:`import { tracewayDebugIds } from "@tracewayapp/bundler-plugin/rollup";

export default {
  output: {
    sourcemap: true,
  },
  plugins: [tracewayDebugIds()],
};`},webpack:{label:"webpack",file:"webpack.config.js",directory:"dist",language:bt,code:`const {
  TracewayDebugIdsWebpackPlugin,
} = require("@tracewayapp/bundler-plugin/webpack");

module.exports = {
  devtool: "source-map",
  plugins: [new TracewayDebugIdsWebpackPlugin()],
};`}};let a=le("vite"),l=le(!1),v=le(!1),d=le(!1),_=le(!1),b=le(!1),y=le(!1),T=le(!1),A=le(!1),p=le(!1),w=le(!1),E=le(!1);const m="npm install -D @tracewayapp/bundler-plugin",u=z(()=>it.currentProject),h=z(()=>r(u)?.sourceMapToken??null),$=z(()=>Kt.getRoleForOrganization(r(u)?.organizationId??0)==="readonly"),S=z(()=>r(u)?.framework==="flutter"),x=z(()=>r(u)?.framework==="ios"),C=z(()=>r(u)?.framework==="android"),I=z(()=>r(S)||r(x)||r(C)?"debug symbols":"source maps"),P=z(()=>r(u)?.framework!=="react-native"),B=z(()=>r(u)&&r(h)?`npx @tracewayapp/sourcemap-upload \\
  --url ${r(u).backendUrl} \\
  --token ${r(h)} \\
  --directory ${r(P)?n[r(a)].directory:"dist"}`:""),re="flutter build apk --release --obfuscate --split-debug-info=build/symbols",oe=z(()=>r(u)&&r(h)?`dart run traceway:upload_symbols \\
  --token ${r(h)} \\
  --url ${r(u).backendUrl}`:""),ge=`xcodebuild -scheme MyApp -configuration Release \\
  -archivePath build/MyApp.xcarchive archive`,we=z(()=>r(u)&&r(h)?`curl -X POST ${r(u).backendUrl}/api/symbols/upload \\
  -H "Authorization: Bearer ${r(h)}" \\
  -F "files=@build/MyApp.xcarchive/dSYMs/MyApp.app.dSYM/Contents/Resources/DWARF/MyApp"`:""),ve=z(()=>r(u)&&r(h)?`plugins {
  id("com.tracewayapp.symbols")
}

android {
  buildTypes {
    release { isMinifyEnabled = true }
  }
}

traceway {
  token = "${r(h)}"
  url = "${r(u).backendUrl}"
}`:""),W="./gradlew assembleRelease uploadReleaseTracewaySymbols";let D=le(!1);async function Z(){N(l,!0);try{await it.generateSourceMapToken()}finally{N(l,!1)}}async function se(){N(l,!0);try{await it.generateSourceMapToken(),N(D,!1),ya.success("Successfully regenerated the Upload Token",{position:"top-center"})}finally{N(l,!1)}}async function O(){r(h)&&(await navigator.clipboard.writeText(r(h)),N(v,!0),setTimeout(()=>N(v,!1),2e3))}async function F(){await navigator.clipboard.writeText(m),N(d,!0),setTimeout(()=>N(d,!1),2e3)}async function J(){await navigator.clipboard.writeText(n[r(a)].code),N(_,!0),setTimeout(()=>N(_,!1),2e3)}async function ue(){await navigator.clipboard.writeText(r(B)),N(b,!0),setTimeout(()=>N(b,!1),2e3)}async function ie(){await navigator.clipboard.writeText(re),N(y,!0),setTimeout(()=>N(y,!1),2e3)}async function _e(){await navigator.clipboard.writeText(r(oe)),N(T,!0),setTimeout(()=>N(T,!1),2e3)}async function tt(){await navigator.clipboard.writeText(ge),N(A,!0),setTimeout(()=>N(A,!1),2e3)}async function gt(){await navigator.clipboard.writeText(r(we)),N(p,!0),setTimeout(()=>N(p,!1),2e3)}async function vt(){await navigator.clipboard.writeText(r(ve)),N(w,!0),setTimeout(()=>N(w,!1),2e3)}async function oa(){await navigator.clipboard.writeText(W),N(E,!0),setTimeout(()=>N(E,!1),2e3)}var Nt=un(),Ct=s(Nt);{var sa=Qe=>{var je=jr(),Ze=c(je),ct=g(c(Ze),2),Je=c(ct),ze=c(Je,!0);i(Je);var Me=g(Je,2);me(Me,{variant:"outline",size:"sm",onclick:O,children:(q,ye)=>{var ee=L(),fe=s(ee);{var Ee=M=>{Ie(M,{class:"h-4 w-4 text-green-500"})},Y=M=>{$e(M,{class:"h-4 w-4"})};k(fe,M=>{r(v)?M(Ee):M(Y,!1)})}o(q,ee)},$$slots:{default:!0}});var et=g(Me,2);me(et,{variant:"destructiveOutline",size:"sm",onclick:()=>N(D,!0),children:(q,ye)=>{var ee=Rr(),fe=s(ee);Ua(fe,{class:"mr-2 h-4 w-4"}),R(),o(q,ee)},$$slots:{default:!0}}),i(ct),i(Ze);var lt=g(Ze,2);{var _t=q=>{var ye=Mr(),ee=s(ye),fe=g(c(ee),2),Ee=c(fe),Y=c(Ee);me(Y,{variant:"outline",size:"sm",onclick:ie,children:(Re,De)=>{var Ne=L(),Le=s(Ne);{var ke=G=>{var V=Nr(),Q=s(V);Ie(Q,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(G,V)},Pe=G=>{var V=Cr(),Q=s(V);$e(Q,{class:"mr-2 h-4 w-4"}),R(),o(G,V)};k(Le,G=>{r(y)?G(ke):G(Pe,!1)})}o(Re,Ne)},$$slots:{default:!0}}),i(Ee);var M=g(Ee,2),X=c(M);qe(X,{get language(){return Ve},code:re}),i(M),i(fe),R(2),i(ee);var pe=g(ee,2),ce=g(c(pe),2),K=c(ce),U=c(K);me(U,{variant:"outline",size:"sm",onclick:_e,children:(Re,De)=>{var Ne=L(),Le=s(Ne);{var ke=G=>{var V=Ir(),Q=s(V);Ie(Q,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(G,V)},Pe=G=>{var V=$r(),Q=s(V);$e(Q,{class:"mr-2 h-4 w-4"}),R(),o(G,V)};k(Le,G=>{r(T)?G(ke):G(Pe,!1)})}o(Re,Ne)},$$slots:{default:!0}}),i(K);var be=g(K,2),Oe=c(be);qe(Oe,{get language(){return Ve},get code(){return r(oe)}}),i(be),i(ce),R(2),i(pe),j(()=>{He(M,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`),He(be,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`)}),o(q,ye)},nt=q=>{var ye=L(),ee=s(ye);{var fe=Y=>{var M=Br(),X=s(M),pe=g(c(X),2),ce=c(pe),K=c(ce);me(K,{variant:"outline",size:"sm",onclick:tt,children:(Pe,G)=>{var V=L(),Q=s(V);{var Ye=te=>{var H=Lr(),he=s(H);Ie(he,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(te,H)},Se=te=>{var H=kr(),he=s(H);$e(he,{class:"mr-2 h-4 w-4"}),R(),o(te,H)};k(Q,te=>{r(A)?te(Ye):te(Se,!1)})}o(Pe,V)},$$slots:{default:!0}}),i(ce);var U=g(ce,2),be=c(U);qe(be,{get language(){return Ve},code:ge}),i(U),i(pe),R(2),i(X);var Oe=g(X,2),Re=g(c(Oe),2),De=c(Re),Ne=c(De);me(Ne,{variant:"outline",size:"sm",onclick:gt,children:(Pe,G)=>{var V=L(),Q=s(V);{var Ye=te=>{var H=Pr(),he=s(H);Ie(he,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(te,H)},Se=te=>{var H=Dr(),he=s(H);$e(he,{class:"mr-2 h-4 w-4"}),R(),o(te,H)};k(Q,te=>{r(p)?te(Ye):te(Se,!1)})}o(Pe,V)},$$slots:{default:!0}}),i(De);var Le=g(De,2),ke=c(Le);qe(ke,{get language(){return Ve},get code(){return r(we)}}),i(Le),i(Re),R(2),i(Oe),j(()=>{He(U,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`),He(Le,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`)}),o(Y,M)},Ee=Y=>{var M=L(),X=s(M);{var pe=K=>{var U=Gr(),be=s(U),Oe=g(c(be),2),Re=c(Oe),De=c(Re);me(De,{variant:"outline",size:"sm",onclick:vt,children:(Se,te)=>{var H=L(),he=s(H);{var xe=ae=>{var Ae=Ur(),Ue=s(Ae);Ie(Ue,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(ae,Ae)},Be=ae=>{var Ae=Fr(),Ue=s(Ae);$e(Ue,{class:"mr-2 h-4 w-4"}),R(),o(ae,Ae)};k(he,ae=>{r(w)?ae(xe):ae(Be,!1)})}o(Se,H)},$$slots:{default:!0}}),i(Re);var Ne=g(Re,2),Le=c(Ne);qe(Le,{get language(){return bt},get code(){return r(ve)}}),i(Ne),i(Oe),R(2),i(be);var ke=g(be,2),Pe=g(c(ke),2),G=c(Pe),V=c(G);me(V,{variant:"outline",size:"sm",onclick:oa,children:(Se,te)=>{var H=L(),he=s(H);{var xe=ae=>{var Ae=Kr(),Ue=s(Ae);Ie(Ue,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(ae,Ae)},Be=ae=>{var Ae=zr(),Ue=s(Ae);$e(Ue,{class:"mr-2 h-4 w-4"}),R(),o(ae,Ae)};k(he,ae=>{r(E)?ae(xe):ae(Be,!1)})}o(Se,H)},$$slots:{default:!0}}),i(G);var Q=g(G,2),Ye=c(Q);qe(Ye,{get language(){return Ve},code:W}),i(Q),i(Pe),R(2),i(ke),j(()=>{He(Ne,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`),He(Q,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`)}),o(K,U)},ce=K=>{var U=Qr(),be=s(U);{var Oe=Q=>{var Ye=Yr(),Se=s(Ye),te=g(c(Se),2),H=c(te),he=c(H);me(he,{variant:"outline",size:"sm",onclick:F,children:(dt,St)=>{var Ge=L(),ft=s(Ge);{var ot=Te=>{var Ce=Hr(),Xe=s(Ce);Ie(Xe,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(Te,Ce)},ut=Te=>{var Ce=qr(),Xe=s(Ce);$e(Xe,{class:"mr-2 h-4 w-4"}),R(),o(Te,Ce)};k(ft,Te=>{r(d)?Te(ot):Te(ut,!1)})}o(dt,Ge)},$$slots:{default:!0}}),i(H);var xe=g(H,2),Be=c(xe);qe(Be,{get language(){return Ve},code:m}),i(xe),i(te),i(Se);var ae=g(Se,2),Ae=g(c(ae),2);ne(Ae,()=>Et,(dt,St)=>{St(dt,{get value(){return r(a)},onValueChange:Ge=>{Ge&&N(a,Ge,!0)},children:(Ge,ft)=>{var ot=L(),ut=s(ot);ne(ut,()=>yt,(Te,Ce)=>{Ce(Te,{class:"mb-2",children:(Xe,xn)=>{var $t=L(),pa=s($t);pt(pa,17,()=>Object.entries(n),([xt,Mt])=>xt,(xt,Mt)=>{var Lt=z(()=>wa(r(Mt),2));let ma=()=>r(Lt)[0],ga=()=>r(Lt)[1];var kt=L(),va=s(kt);ne(va,()=>mt,(_a,fa)=>{fa(_a,{get value(){return ma()},children:(ba,An)=>{R();var Pt=Ke();j(()=>de(Pt,ga().label)),o(ba,Pt)},$$slots:{default:!0}})}),o(xt,kt)}),o(Xe,$t)},$$slots:{default:!0}})}),o(Ge,ot)},$$slots:{default:!0}})});var Ue=g(Ae,2),la=c(Ue,!0);i(Ue);var It=g(Ue,2),Tt=c(It),da=c(Tt);me(da,{variant:"outline",size:"sm",onclick:J,children:(dt,St)=>{var Ge=L(),ft=s(Ge);{var ot=Te=>{var Ce=Wr(),Xe=s(Ce);Ie(Xe,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(Te,Ce)},ut=Te=>{var Ce=Zr(),Xe=s(Ce);$e(Xe,{class:"mr-2 h-4 w-4"}),R(),o(Te,Ce)};k(ft,Te=>{r(_)?Te(ot):Te(ut,!1)})}o(dt,Ge)},$$slots:{default:!0}}),i(Tt);var wt=g(Tt,2),ua=c(wt);qe(ua,{get language(){return n[r(a)].language},get code(){return n[r(a)].code}}),i(wt),i(It),i(ae),j(()=>{He(xe,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`),de(la,n[r(a)].file),He(wt,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`)}),o(Q,Ye)};k(be,Q=>{r(P)&&Q(Oe)})}var Re=g(be,2),De=c(Re),Ne=c(De,!0);i(De);var Le=g(De,2),ke=c(Le),Pe=c(ke);me(Pe,{variant:"outline",size:"sm",onclick:ue,children:(Q,Ye)=>{var Se=L(),te=s(Se);{var H=xe=>{var Be=Xr(),ae=s(Be);Ie(ae,{class:"mr-2 h-4 w-4 text-green-500"}),R(),o(xe,Be)},he=xe=>{var Be=Vr(),ae=s(Be);$e(ae,{class:"mr-2 h-4 w-4"}),R(),o(xe,Be)};k(te,xe=>{r(b)?xe(H):xe(he,!1)})}o(Q,Se)},$$slots:{default:!0}}),i(ke);var G=g(ke,2),V=c(G);qe(V,{get language(){return Ve},get code(){return r(B)}}),i(G),i(Le),i(Re),j(()=>{de(Ne,r(P)?"Step 3: Upload after your production build":"Usage"),He(G,1,`overflow-x-auto rounded-lg text-sm ${We.isDark?"dark-code":"light-code"}`)}),o(K,U)};k(X,K=>{r(C)?K(pe):K(ce,!1)},!0)}o(Y,M)};k(ee,Y=>{r(x)?Y(fe):Y(Ee,!1)},!0)}o(q,ye)};k(lt,q=>{r(S)?q(_t):q(nt,!1)})}i(je),j(()=>de(ze,r(h))),o(Qe,je)},ia=Qe=>{var je=L(),Ze=s(je);{var ct=ze=>{var Me=Jr(),et=c(Me);i(Me),j(()=>de(et,`An upload token is required to upload ${r(I)??""}. Ask an organization admin to generate one
		from the Connection page.`)),o(ze,Me)},Je=ze=>{var Me=sn(),et=c(Me);{var lt=q=>{var ye=en();o(q,ye)},_t=q=>{var ye=L(),ee=s(ye);{var fe=Y=>{var M=tn();o(Y,M)},Ee=Y=>{var M=L(),X=s(M);{var pe=K=>{var U=an();o(K,U)},ce=K=>{var U=rn(),be=c(U);i(U),j(()=>de(be,`Generate an upload token to start uploading ${r(I)??""} as part of your build process.`)),o(K,U)};k(X,K=>{r(C)?K(pe):K(ce,!1)},!0)}o(Y,M)};k(ee,Y=>{r(x)?Y(fe):Y(Ee,!1)},!0)}o(q,ye)};k(et,q=>{r(S)?q(lt):q(_t,!1)})}var nt=g(et,2);me(nt,{variant:"outline",size:"sm",onclick:Z,get disabled(){return r(l)},children:(q,ye)=>{var ee=L(),fe=s(ee);{var Ee=M=>{var X=nn(),pe=s(X);$a(pe,{class:"mr-2 h-4 w-4"}),R(),o(M,X)},Y=M=>{var X=on(),pe=s(X);zt(pe,{class:"mr-2 h-4 w-4"}),R(),o(M,X)};k(fe,M=>{r(l)?M(Ee):M(Y,!1)})}o(q,ee)},$$slots:{default:!0}}),i(Me),o(ze,Me)};k(Ze,ze=>{r($)?ze(ct):ze(Je,!1)},!0)}o(Qe,je)};k(Ct,Qe=>{r(h)?Qe(sa):Qe(ia,!1)})}var ca=g(Ct,2);ne(ca,()=>Ba,(Qe,je)=>{je(Qe,{get open(){return r(D)},set open(Ze){N(D,Ze,!0)},children:(Ze,ct)=>{var Je=L(),ze=s(Je);ne(ze,()=>Ma,(Me,et)=>{et(Me,{interactOutsideBehavior:"close",children:(lt,_t)=>{var nt=dn(),q=s(nt);ne(q,()=>La,(ee,fe)=>{fe(ee,{children:(Ee,Y)=>{var M=cn(),X=s(M);ne(X,()=>ka,(ce,K)=>{K(ce,{children:(U,be)=>{R();var Oe=Ke("Regenerate Upload Token");o(U,Oe)},$$slots:{default:!0}})});var pe=g(X,2);ne(pe,()=>Pa,(ce,K)=>{K(ce,{children:(U,be)=>{R();var Oe=Ke(`A new upload token will be issued for this project and the current one will stop working
				immediately.`);o(U,Oe)},$$slots:{default:!0}})}),o(Ee,M)},$$slots:{default:!0}})});var ye=g(q,4);ne(ye,()=>Da,(ee,fe)=>{fe(ee,{class:"sm:justify-between",children:(Ee,Y)=>{var M=ln(),X=s(M);me(X,{variant:"outline",onclick:()=>N(D,!1),get disabled(){return r(l)},children:(ce,K)=>{R();var U=Ke("Cancel");o(ce,U)},$$slots:{default:!0}});var pe=g(X,2);me(pe,{variant:"destructive",onclick:se,get disabled(){return r(l)},children:(ce,K)=>{R();var U=Ke();j(()=>de(U,r(l)?"Regenerating...":"Regenerate Token")),o(ce,U)},$$slots:{default:!0}}),o(Ee,M)},$$slots:{default:!0}})}),o(lt,nt)},$$slots:{default:!0}})}),o(Ze,Je)},$$slots:{default:!0}})}),o(e,Nt),rt()}var mn=f("<!> ",1),gn=f("<!> <!>",1),vn=f("<!> <!>",1);function _n(e,t){at(t,!0);let n=z(()=>it.currentProject);const a=z(()=>r(n)?.framework==="flutter"),l=z(()=>r(n)?.framework==="ios"),v=z(()=>Kt.getRoleForOrganization(it.currentProject?.organizationId??0)==="readonly");var d=L(),_=s(d);{var b=y=>{Oa(y,{children:(T,A)=>{var p=vn(),w=s(p);Ra(w,{children:(m,u)=>{var h=gn(),$=s(h);Na($,{class:"flex items-center gap-2",children:(C,I)=>{var P=mn(),B=s(P);zt(B,{class:"h-5 w-5"});var re=g(B);j(()=>de(re,` ${r(a)||r(l)?"Symbol Upload":"Source Map Upload"}`)),o(C,P)},$$slots:{default:!0}});var S=g($,2);{var x=C=>{Ia(C,{children:(I,P)=>{R();var B=Ke(`Upload source maps to see original file names and line numbers in stack traces from
					minified code.`);o(I,B)},$$slots:{default:!0}})};k(S,C=>{!r(a)&&!r(l)&&C(x)})}o(m,h)},$$slots:{default:!0}});var E=g(w,2);Ca(E,{children:(m,u)=>{pn(m,{})},$$slots:{default:!0}}),o(T,p)},$$slots:{default:!0}})};k(_,y=>{r(n)&&!r(v)&&y(b)})}o(e,d),rt()}var fn=f('<p class="pt-1 text-sm font-medium">Framework</p> <!>',1),bn=f('<p class="mt-1 ml-9 text-sm text-muted-foreground"> </p>'),yn=f('<p class="pt-2 text-xs text-muted-foreground"><a target="_blank" rel="noopener noreferrer" class="underline hover:text-foreground"> </a></p>'),En=f('<div class="p-4"><!> <!></div>'),hn=f('<div class="rounded-md border bg-card"><div class="border-b px-4 py-3"><div class="flex items-center gap-3"><div class="flex h-6 w-6 items-center justify-center rounded-full bg-primary text-sm font-medium text-primary-foreground"> </div> <h3 class="font-semibold"> </h3></div> <!></div> <!></div>'),Tn=f('<div class="space-y-2"><p class="text-sm font-medium">Language</p> <!> <!></div> <!> <!>',1);function ao(e,t){at(t,!0);let n=le(Dt(rr())),a=le(Dt(or()));const l={bash:Ve,go:Aa,javascript:bt,typescript:ra,python:Va,gradle:Tr,csharp:Sr,ruby:Ar,yaml:na},v=z(()=>st.find(S=>S.id===r(n))??st[0]),d=z(()=>r(v).frameworks.find(S=>S.id===r(a))?.id??r(v).frameworks[0]?.id??""),_=z(()=>tr(r(v).id,r(d),t.backendUrl,t.token));function b(S){const x=st.find(C=>C.id===S);x&&(N(n,x.id,!0),nr(x.id))}function y(S){r(v).frameworks.some(x=>x.id===S)&&(N(a,S,!0),sr(S))}function T(S){return l[S??"bash"]}var A=Tn(),p=s(A),w=g(c(p),2);ne(w,()=>Et,(S,x)=>{x(S,{get value(){return r(n)},onValueChange:b,children:(C,I)=>{var P=L(),B=s(P);ne(B,()=>yt,(re,oe)=>{oe(re,{class:"h-auto flex-wrap justify-start",children:(ge,we)=>{var ve=L(),W=s(ve);pt(W,17,()=>st,D=>D.id,(D,Z)=>{var se=L(),O=s(se);ne(O,()=>mt,(F,J)=>{J(F,{get value(){return r(Z).id},children:(ue,ie)=>{R();var _e=Ke();j(()=>de(_e,r(Z).label)),o(ue,_e)},$$slots:{default:!0}})}),o(D,se)}),o(ge,ve)},$$slots:{default:!0}})}),o(C,P)},$$slots:{default:!0}})});var E=g(w,2);{var m=S=>{var x=fn(),C=g(s(x),2);ne(C,()=>Et,(I,P)=>{P(I,{get value(){return r(d)},onValueChange:y,children:(B,re)=>{var oe=L(),ge=s(oe);ne(ge,()=>yt,(we,ve)=>{ve(we,{class:"h-auto flex-wrap justify-start",children:(W,D)=>{var Z=L(),se=s(Z);pt(se,17,()=>r(v).frameworks,O=>O.id,(O,F)=>{var J=L(),ue=s(J);ne(ue,()=>mt,(ie,_e)=>{_e(ie,{get value(){return r(F).id},children:(tt,gt)=>{R();var vt=Ke();j(()=>de(vt,r(F).label)),o(tt,vt)},$$slots:{default:!0}})}),o(O,J)}),o(W,Z)},$$slots:{default:!0}})}),o(B,oe)},$$slots:{default:!0}})}),o(S,x)};k(E,S=>{r(v).frameworks.length>1&&S(m)})}i(p);var u=g(p,2);pt(u,19,()=>r(_),S=>r(v).id+r(d)+S.title,(S,x,C)=>{var I=hn(),P=c(I),B=c(P),re=c(B),oe=c(re,!0);i(re);var ge=g(re,2),we=c(ge,!0);i(ge),i(B);var ve=g(B,2);{var W=se=>{var O=bn(),F=c(O,!0);i(O),j(()=>de(F,r(x).description)),o(se,O)};k(ve,se=>{r(x).description&&se(W)})}i(P);var D=g(P,2);{var Z=se=>{var O=En(),F=c(O);{let ie=z(()=>T(r(x).codeLanguage));Rt(F,{get code(){return r(x).code},get language(){return r(ie)}})}var J=g(F,2);{var ue=ie=>{var _e=yn(),tt=c(_e),gt=c(tt,!0);i(tt),i(_e),j(()=>{xa(tt,"href",r(x).link.href),de(gt,r(x).link.label)}),o(ie,_e)};k(J,ie=>{r(x).link&&ie(ue)})}i(O),o(se,O)};k(D,se=>{r(x).code&&se(Z)})}i(I),j(()=>{de(oe,r(C)+1),de(we,r(x).title)}),o(S,I)});var h=g(u,2);{var $=S=>{_n(S,{})};k(h,S=>{r(n)==="nodejs"&&S($)})}o(e,A),rt()}var wn=f('<div class="flex items-center gap-2"><code class="flex-1 rounded-md bg-muted px-3 py-2 font-mono text-sm break-all"> </code> <!></div>');function Ft(e,t){let n=le(!1);async function a(){await navigator.clipboard.writeText(t.value),N(n,!0),setTimeout(()=>N(n,!1),2e3)}var l=wn(),v=c(l),d=c(v,!0);i(v);var _=g(v,2);me(_,{variant:"outline",size:"sm",onclick:a,children:(b,y)=>{var T=L(),A=s(T);{var p=E=>{Ie(E,{class:"h-4 w-4 text-green-500"})},w=E=>{$e(E,{class:"h-4 w-4"})};k(A,E=>{r(n)?E(p):E(w,!1)})}o(b,T)},$$slots:{default:!0}}),i(l),j(()=>de(d,t.value)),o(e,l)}var Sn=f('<div class="space-y-6"><div><p class="mb-1 text-sm font-medium">OTLP Endpoint</p> <p class="mb-2 text-xs text-muted-foreground">Your SDK or Collector will append <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/traces</code> and <code class="rounded bg-muted px-1 py-0.5 font-mono text-xs">/v1/metrics</code> automatically.</p> <!></div> <div><p class="mb-2 text-sm font-medium">Authorization Header</p> <!></div> <div><p class="mb-2 text-sm font-medium">Example: OTel Collector (optional)</p> <!></div></div>');function ro(e,t){var n=Sn(),a=c(n),l=g(c(a),4);Ft(l,{get value(){return t.endpoint}}),i(a);var v=g(a,2),d=g(c(v),2);Ft(d,{get value(){return t.authHeader}}),i(v);var _=g(v,2),b=g(c(_),2);Rt(b,{get code(){return t.collectorConfig},get language(){return na}}),i(_),i(n),o(e,n)}export{to as A,ao as O,eo as S,ro as a,Qn as b,Ve as c,Yn as d,Va as e,Zn as f,Jn as g,_n as h,Xn as i,bt as j,Vn as k,jn as l,Wn as p};
