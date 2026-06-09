import{f as U}from"./Brfw0Axo.js";const H="[A-Za-z$_][0-9A-Za-z$_]*",q=["as","in","of","if","for","while","finally","var","new","function","do","return","void","else","break","catch","instanceof","with","throw","case","default","try","switch","continue","typeof","delete","let","yield","const","class","debugger","async","await","static","import","from","export","extends","using"],Z=["true","false","null","undefined","NaN","Infinity"],F=["Object","Function","Boolean","Symbol","Math","Date","Number","BigInt","String","RegExp","Array","Float32Array","Float64Array","Int8Array","Uint8Array","Uint8ClampedArray","Int16Array","Int32Array","Uint16Array","Uint32Array","BigInt64Array","BigUint64Array","Set","Map","WeakSet","WeakMap","ArrayBuffer","SharedArrayBuffer","Atomics","DataView","JSON","Promise","Generator","GeneratorFunction","AsyncFunction","Reflect","Proxy","Intl","WebAssembly"],G=["Error","EvalError","InternalError","RangeError","ReferenceError","SyntaxError","TypeError","URIError"],K=["setInterval","setTimeout","clearInterval","clearTimeout","require","exports","eval","isFinite","isNaN","parseFloat","parseInt","decodeURI","decodeURIComponent","encodeURI","encodeURIComponent","escape","unescape"],X=["arguments","this","super","console","window","document","localStorage","sessionStorage","module","global"],W=[].concat(K,F,G);function V(e){const t=e.regex,n=(_,{after:v})=>{const C="</"+_[0].slice(1);return _.input.indexOf(C,v)!==-1},a=H,p={begin:"<>",end:"</>"},f=/<[A-Za-z0-9\\._:-]+\s*\/>/,l={begin:/<[A-Za-z0-9\\._:-]+/,end:/\/[A-Za-z0-9\\._:-]+>|\/>/,isTrulyOpeningTag:(_,v)=>{const C=_[0].length+_.index,D=_.input[C];if(D==="<"||D===","){v.ignoreMatch();return}D===">"&&(n(_,{after:C})||v.ignoreMatch());let B;const k=_.input.substring(C);if(B=k.match(/^\s*=/)){v.ignoreMatch();return}if((B=k.match(/^\s+extends\s+/))&&B.index===0){v.ignoreMatch();return}}},o={$pattern:H,keyword:q,literal:Z,built_in:W,"variable.language":X},s="[0-9](_?[0-9])*",u=`\\.(${s})`,g="0|[1-9](_?[0-9])*|0[0-7]*[89][0-9]*",y={className:"number",variants:[{begin:`(\\b(${g})((${u})|\\.)?|(${u}))[eE][+-]?(${s})\\b`},{begin:`\\b(${g})\\b((${u})\\b|\\.)?|(${u})\\b`},{begin:"\\b(0|[1-9](_?[0-9])*)n\\b"},{begin:"\\b0[xX][0-9a-fA-F](_?[0-9a-fA-F])*n?\\b"},{begin:"\\b0[bB][0-1](_?[0-1])*n?\\b"},{begin:"\\b0[oO][0-7](_?[0-7])*n?\\b"},{begin:"\\b0[0-7]+n?\\b"}],relevance:0},r={className:"subst",begin:"\\$\\{",end:"\\}",keywords:o,contains:[]},d={begin:".?html`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,r],subLanguage:"xml"}},c={begin:".?css`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,r],subLanguage:"css"}},i={begin:".?gql`",end:"",starts:{end:"`",returnEnd:!1,contains:[e.BACKSLASH_ESCAPE,r],subLanguage:"graphql"}},m={className:"string",begin:"`",end:"`",contains:[e.BACKSLASH_ESCAPE,r]},w={className:"comment",variants:[e.COMMENT(/\/\*\*(?!\/)/,"\\*/",{relevance:0,contains:[{begin:"(?=@[A-Za-z]+)",relevance:0,contains:[{className:"doctag",begin:"@[A-Za-z]+"},{className:"type",begin:"\\{",end:"\\}",excludeEnd:!0,excludeBegin:!0,relevance:0},{className:"variable",begin:a+"(?=\\s*(-)|$)",endsParent:!0,relevance:0},{begin:/(?=[^\n])\s/,relevance:0}]}]}),e.C_BLOCK_COMMENT_MODE,e.C_LINE_COMMENT_MODE]},O=[e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,d,c,i,m,{match:/\$\d+/},y];r.contains=O.concat({begin:/\{/,end:/\}/,keywords:o,contains:["self"].concat(O)});const R=[].concat(w,r.contains),S=R.concat([{begin:/(\s*)\(/,end:/\)/,keywords:o,contains:["self"].concat(R)}]),h={className:"params",begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,contains:S},I={variants:[{match:[/class/,/\s+/,a,/\s+/,/extends/,/\s+/,t.concat(a,"(",t.concat(/\./,a),")*")],scope:{1:"keyword",3:"title.class",5:"keyword",7:"title.class.inherited"}},{match:[/class/,/\s+/,a],scope:{1:"keyword",3:"title.class"}}]},N={relevance:0,match:t.either(/\bJSON/,/\b[A-Z][a-z]+([A-Z][a-z]*|\d)*/,/\b[A-Z]{2,}([A-Z][a-z]+|\d)+([A-Z][a-z]*)*/,/\b[A-Z]{2,}[a-z]+([A-Z][a-z]+|\d)*([A-Z][a-z]*)*/),className:"title.class",keywords:{_:[...F,...G]}},x={label:"use_strict",className:"meta",relevance:10,begin:/^\s*['"]use (strict|asm)['"]/},L={variants:[{match:[/function/,/\s+/,a,/(?=\s*\()/]},{match:[/function/,/\s*(?=\()/]}],className:{1:"keyword",3:"title.function"},label:"func.def",contains:[h],illegal:/%/},$={relevance:0,match:/\b[A-Z][A-Z_0-9]+\b/,className:"variable.constant"};function M(_){return t.concat("(?!",_.join("|"),")")}const P={match:t.concat(/\b/,M([...K,"super","import"].map(_=>`${_}\\s*\\(`)),a,t.lookahead(/\s*\(/)),className:"title.function",relevance:0},T={begin:t.concat(/\./,t.lookahead(t.concat(a,/(?![0-9A-Za-z$_(])/))),end:a,excludeBegin:!0,keywords:"prototype",className:"property",relevance:0},E={match:[/get|set/,/\s+/,a,/(?=\()/],className:{1:"keyword",3:"title.function"},contains:[{begin:/\(\)/},h]},A="(\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)|"+e.UNDERSCORE_IDENT_RE+")\\s*=>",z={match:[/const|var|let/,/\s+/,a,/\s*/,/=\s*/,/(async\s*)?/,t.lookahead(A)],keywords:"async",className:{1:"keyword",3:"title.function"},contains:[h]};return{name:"JavaScript",aliases:["js","jsx","mjs","cjs"],keywords:o,exports:{PARAMS_CONTAINS:S,CLASS_REFERENCE:N},illegal:/#(?![$_A-z])/,contains:[e.SHEBANG({label:"shebang",binary:"node",relevance:5}),x,e.APOS_STRING_MODE,e.QUOTE_STRING_MODE,d,c,i,m,w,{match:/\$\d+/},y,N,{scope:"attr",match:a+t.lookahead(":"),relevance:0},z,{begin:"("+e.RE_STARTERS_RE+"|\\b(case|return|throw)\\b)\\s*",keywords:"return throw case",relevance:0,contains:[w,e.REGEXP_MODE,{className:"function",begin:A,returnBegin:!0,end:"\\s*=>",contains:[{className:"params",variants:[{begin:e.UNDERSCORE_IDENT_RE,relevance:0},{className:null,begin:/\(\s*\)/,skip:!0},{begin:/(\s*)\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,contains:S}]}]},{begin:/,/,relevance:0},{match:/\s+/,relevance:0},{variants:[{begin:p.begin,end:p.end},{match:f},{begin:l.begin,"on:begin":l.isTrulyOpeningTag,end:l.end}],subLanguage:"xml",contains:[{begin:l.begin,end:l.end,skip:!0,contains:["self"]}]}]},L,{beginKeywords:"while if switch catch for"},{begin:"\\b(?!function)"+e.UNDERSCORE_IDENT_RE+"\\([^()]*(\\([^()]*(\\([^()]*\\)[^()]*)*\\)[^()]*)*\\)\\s*\\{",returnBegin:!0,label:"func.def",contains:[h,e.inherit(e.TITLE_MODE,{begin:a,className:"title.function"})]},{match:/\.\.\./,relevance:0},T,{match:"\\$"+a,relevance:0},{match:[/\bconstructor(?=\s*\()/],className:{1:"title.function"},contains:[h]},P,$,I,E,{match:/\$[(.]/}]}}const ae={name:"javascript",register:V};function j(e){const t=e.regex,n={},a={begin:/\$\{/,end:/\}/,contains:["self",{begin:/:-/,contains:[n]}]};Object.assign(n,{className:"variable",variants:[{begin:t.concat(/\$[\w\d#@][\w\d_]*/,"(?![\\w\\d])(?![$])")},a]});const p={className:"subst",begin:/\$\(/,end:/\)/,contains:[e.BACKSLASH_ESCAPE]},f=e.inherit(e.COMMENT(),{match:[/(^|\s)/,/#.*$/],scope:{2:"comment"}}),l={begin:/<<-?\s*(?=\w+)/,starts:{contains:[e.END_SAME_AS_BEGIN({begin:/(\w+)/,end:/(\w+)/,className:"string"})]}},o={className:"string",begin:/"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,n,p]};p.contains.push(o);const s={match:/\\"/},u={className:"string",begin:/'/,end:/'/},g={match:/\\'/},y={begin:/\$?\(\(/,end:/\)\)/,contains:[{begin:/\d+#[0-9a-f]+/,className:"number"},e.NUMBER_MODE,n]},r=["fish","bash","zsh","sh","csh","ksh","tcsh","dash","scsh"],d=e.SHEBANG({binary:`(${r.join("|")})`,relevance:10}),c={className:"function",begin:/\w[\w\d_]*\s*\(\s*\)\s*\{/,returnBegin:!0,contains:[e.inherit(e.TITLE_MODE,{begin:/\w[\w\d_]*/})],relevance:0},i=["if","then","else","elif","fi","time","for","while","until","in","do","done","case","esac","coproc","function","select"],m=["true","false"],b={match:/(\/[a-z._-]+)+/},w=["break","cd","continue","eval","exec","exit","export","getopts","hash","pwd","readonly","return","shift","test","times","trap","umask","unset"],O=["alias","bind","builtin","caller","command","declare","echo","enable","help","let","local","logout","mapfile","printf","read","readarray","source","sudo","type","typeset","ulimit","unalias"],R=["autoload","bg","bindkey","bye","cap","chdir","clone","comparguments","compcall","compctl","compdescribe","compfiles","compgroups","compquote","comptags","comptry","compvalues","dirs","disable","disown","echotc","echoti","emulate","fc","fg","float","functions","getcap","getln","history","integer","jobs","kill","limit","log","noglob","popd","print","pushd","pushln","rehash","sched","setcap","setopt","stat","suspend","ttyctl","unfunction","unhash","unlimit","unsetopt","vared","wait","whence","where","which","zcompile","zformat","zftp","zle","zmodload","zparseopts","zprof","zpty","zregexparse","zsocket","zstyle","ztcp"],S=["chcon","chgrp","chown","chmod","cp","dd","df","dir","dircolors","ln","ls","mkdir","mkfifo","mknod","mktemp","mv","realpath","rm","rmdir","shred","sync","touch","truncate","vdir","b2sum","base32","base64","cat","cksum","comm","csplit","cut","expand","fmt","fold","head","join","md5sum","nl","numfmt","od","paste","ptx","pr","sha1sum","sha224sum","sha256sum","sha384sum","sha512sum","shuf","sort","split","sum","tac","tail","tr","tsort","unexpand","uniq","wc","arch","basename","chroot","date","dirname","du","echo","env","expr","factor","groups","hostid","id","link","logname","nice","nohup","nproc","pathchk","pinky","printenv","printf","pwd","readlink","runcon","seq","sleep","stat","stdbuf","stty","tee","test","timeout","tty","uname","unlink","uptime","users","who","whoami","yes"];return{name:"Bash",aliases:["sh","zsh"],keywords:{$pattern:/\b[a-z][a-z0-9._-]+\b/,keyword:i,literal:m,built_in:[...w,...O,"set","shopt",...R,...S]},contains:[d,e.SHEBANG(),c,y,f,l,b,o,s,u,g,n]}}const ne={name:"bash",register:j};function Y(e){const t=e.regex,n=/(?![A-Za-z0-9])(?![$])/,a=t.concat(/[a-zA-Z_\x7f-\xff][a-zA-Z0-9_\x7f-\xff]*/,n),p=t.concat(/(\\?[A-Z][a-z0-9_\x7f-\xff]+|\\?[A-Z]+(?=[A-Z][a-z0-9_\x7f-\xff])){1,}/,n),f=t.concat(/[A-Z]+/,n),l={scope:"variable",match:"\\$+"+a},o={scope:"meta",variants:[{begin:/<\?php/,relevance:10},{begin:/<\?=/},{begin:/<\?/,relevance:.1},{begin:/\?>/}]},s={scope:"subst",variants:[{begin:/\$\w+/},{begin:/\{\$/,end:/\}/}]},u=e.inherit(e.APOS_STRING_MODE,{illegal:null}),g=e.inherit(e.QUOTE_STRING_MODE,{illegal:null,contains:e.QUOTE_STRING_MODE.contains.concat(s)}),y={begin:/<<<[ \t]*(?:(\w+)|"(\w+)")\n/,end:/[ \t]*(\w+)\b/,contains:e.QUOTE_STRING_MODE.contains.concat(s),"on:begin":(T,E)=>{E.data._beginMatch=T[1]||T[2]},"on:end":(T,E)=>{E.data._beginMatch!==T[1]&&E.ignoreMatch()}},r=e.END_SAME_AS_BEGIN({begin:/<<<[ \t]*'(\w+)'\n/,end:/[ \t]*(\w+)\b/}),d=`[ 	
]`,c={scope:"string",variants:[g,u,y,r]},i={scope:"number",variants:[{begin:"\\b0[bB][01]+(?:_[01]+)*\\b"},{begin:"\\b0[oO][0-7]+(?:_[0-7]+)*\\b"},{begin:"\\b0[xX][\\da-fA-F]+(?:_[\\da-fA-F]+)*\\b"},{begin:"(?:\\b\\d+(?:_\\d+)*(\\.(?:\\d+(?:_\\d+)*))?|\\B\\.\\d+)(?:[eE][+-]?\\d+)?"}],relevance:0},m=["false","null","true"],b=["__CLASS__","__DIR__","__FILE__","__FUNCTION__","__COMPILER_HALT_OFFSET__","__LINE__","__METHOD__","__NAMESPACE__","__TRAIT__","die","echo","exit","include","include_once","print","require","require_once","array","abstract","and","as","binary","bool","boolean","break","callable","case","catch","class","clone","const","continue","declare","default","do","double","else","elseif","empty","enddeclare","endfor","endforeach","endif","endswitch","endwhile","enum","eval","extends","final","finally","float","for","foreach","from","global","goto","if","implements","instanceof","insteadof","int","integer","interface","isset","iterable","list","match|0","mixed","new","never","object","or","private","protected","public","readonly","real","return","string","switch","throw","trait","try","unset","use","var","void","while","xor","yield"],w=["Error|0","AppendIterator","ArgumentCountError","ArithmeticError","ArrayIterator","ArrayObject","AssertionError","BadFunctionCallException","BadMethodCallException","CachingIterator","CallbackFilterIterator","CompileError","Countable","DirectoryIterator","DivisionByZeroError","DomainException","EmptyIterator","ErrorException","Exception","FilesystemIterator","FilterIterator","GlobIterator","InfiniteIterator","InvalidArgumentException","IteratorIterator","LengthException","LimitIterator","LogicException","MultipleIterator","NoRewindIterator","OutOfBoundsException","OutOfRangeException","OuterIterator","OverflowException","ParentIterator","ParseError","RangeException","RecursiveArrayIterator","RecursiveCachingIterator","RecursiveCallbackFilterIterator","RecursiveDirectoryIterator","RecursiveFilterIterator","RecursiveIterator","RecursiveIteratorIterator","RecursiveRegexIterator","RecursiveTreeIterator","RegexIterator","RuntimeException","SeekableIterator","SplDoublyLinkedList","SplFileInfo","SplFileObject","SplFixedArray","SplHeap","SplMaxHeap","SplMinHeap","SplObjectStorage","SplObserver","SplPriorityQueue","SplQueue","SplStack","SplSubject","SplTempFileObject","TypeError","UnderflowException","UnexpectedValueException","UnhandledMatchError","ArrayAccess","BackedEnum","Closure","Fiber","Generator","Iterator","IteratorAggregate","Serializable","Stringable","Throwable","Traversable","UnitEnum","WeakReference","WeakMap","Directory","__PHP_Incomplete_Class","parent","php_user_filter","self","static","stdClass"],R={keyword:b,literal:(T=>{const E=[];return T.forEach(A=>{E.push(A),A.toLowerCase()===A?E.push(A.toUpperCase()):E.push(A.toLowerCase())}),E})(m),built_in:w},S=T=>T.map(E=>E.replace(/\|\d+$/,"")),h={variants:[{match:[/new/,t.concat(d,"+"),t.concat("(?!",S(w).join("\\b|"),"\\b)"),p],scope:{1:"keyword",4:"title.class"}}]},I=t.concat(a,"\\b(?!\\()"),N={variants:[{match:[t.concat(/::/,t.lookahead(/(?!class\b)/)),I],scope:{2:"variable.constant"}},{match:[/::/,/class/],scope:{2:"variable.language"}},{match:[p,t.concat(/::/,t.lookahead(/(?!class\b)/)),I],scope:{1:"title.class",3:"variable.constant"}},{match:[p,t.concat("::",t.lookahead(/(?!class\b)/))],scope:{1:"title.class"}},{match:[p,/::/,/class/],scope:{1:"title.class",3:"variable.language"}}]},x={scope:"attr",match:t.concat(a,t.lookahead(":"),t.lookahead(/(?!::)/))},L={relevance:0,begin:/\(/,end:/\)/,keywords:R,contains:[x,l,N,e.C_BLOCK_COMMENT_MODE,c,i,h]},$={relevance:0,match:[/\b/,t.concat("(?!fn\\b|function\\b|",S(b).join("\\b|"),"|",S(w).join("\\b|"),"\\b)"),a,t.concat(d,"*"),t.lookahead(/(?=\()/)],scope:{3:"title.function.invoke"},contains:[L]};L.contains.push($);const M=[x,N,e.C_BLOCK_COMMENT_MODE,c,i,h],P={begin:t.concat(/#\[\s*\\?/,t.either(p,f)),beginScope:"meta",end:/]/,endScope:"meta",keywords:{literal:m,keyword:["new","array"]},contains:[{begin:/\[/,end:/]/,keywords:{literal:m,keyword:["new","array"]},contains:["self",...M]},...M,{scope:"meta",variants:[{match:p},{match:f}]}]};return{case_insensitive:!1,keywords:R,contains:[P,e.HASH_COMMENT_MODE,e.COMMENT("//","$"),e.COMMENT("/\\*","\\*/",{contains:[{scope:"doctag",match:"@[A-Za-z]+"}]}),{match:/__halt_compiler\(\);/,keywords:"__halt_compiler",starts:{scope:"comment",end:e.MATCH_NOTHING_RE,contains:[{match:/\?>/,scope:"meta",endsParent:!0}]}},o,{scope:"variable.language",match:/\$this\b/},l,$,N,{match:[/const/,/\s/,a],scope:{1:"keyword",3:"variable.constant"}},h,{scope:"function",relevance:0,beginKeywords:"fn function",end:/[;{]/,excludeEnd:!0,illegal:"[$%\\[]",contains:[{beginKeywords:"use"},e.UNDERSCORE_TITLE_MODE,{begin:"=>",endsParent:!0},{scope:"params",begin:"\\(",end:"\\)",excludeBegin:!0,excludeEnd:!0,keywords:R,contains:["self",P,l,N,e.C_BLOCK_COMMENT_MODE,c,i]}]},{scope:"class",variants:[{beginKeywords:"enum",illegal:/[($"]/},{beginKeywords:"class interface trait",illegal:/[:($"]/}],relevance:0,end:/\{/,excludeEnd:!0,contains:[{beginKeywords:"extends implements"},e.UNDERSCORE_TITLE_MODE]},{beginKeywords:"namespace",relevance:0,end:";",illegal:/[.']/,contains:[e.inherit(e.UNDERSCORE_TITLE_MODE,{scope:"title.class"})]},{beginKeywords:"use",relevance:0,end:";",contains:[{match:/\b(as|const|function)\b/,scope:"keyword"},e.UNDERSCORE_TITLE_MODE]},c,i]}}const re={name:"php",register:Y};function Q(e){const t=e.regex,n=new RegExp("[\\p{XID_Start}_]\\p{XID_Continue}*","u"),a=["and","as","assert","async","await","break","case","class","continue","def","del","elif","else","except","finally","for","from","global","if","import","in","is","lambda","match","nonlocal|10","not","or","pass","raise","return","try","while","with","yield"],o={$pattern:/[A-Za-z]\w+|__\w+__/,keyword:a,built_in:["__import__","abs","all","any","ascii","bin","bool","breakpoint","bytearray","bytes","callable","chr","classmethod","compile","complex","delattr","dict","dir","divmod","enumerate","eval","exec","filter","float","format","frozenset","getattr","globals","hasattr","hash","help","hex","id","input","int","isinstance","issubclass","iter","len","list","locals","map","max","memoryview","min","next","object","oct","open","ord","pow","print","property","range","repr","reversed","round","set","setattr","slice","sorted","staticmethod","str","sum","super","tuple","type","vars","zip"],literal:["__debug__","Ellipsis","False","None","NotImplemented","True"],type:["Any","Callable","Coroutine","Dict","List","Literal","Generic","Optional","Sequence","Set","Tuple","Type","Union"]},s={className:"meta",begin:/^(>>>|\.\.\.) /},u={className:"subst",begin:/\{/,end:/\}/,keywords:o,illegal:/#/},g={begin:/\{\{/,relevance:0},y={className:"string",contains:[e.BACKSLASH_ESCAPE],variants:[{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,s],relevance:10},{begin:/([uU]|[bB]|[rR]|[bB][rR]|[rR][bB])?"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,s],relevance:10},{begin:/([fF][rR]|[rR][fF]|[fF])'''/,end:/'''/,contains:[e.BACKSLASH_ESCAPE,s,g,u]},{begin:/([fF][rR]|[rR][fF]|[fF])"""/,end:/"""/,contains:[e.BACKSLASH_ESCAPE,s,g,u]},{begin:/([uU]|[rR])'/,end:/'/,relevance:10},{begin:/([uU]|[rR])"/,end:/"/,relevance:10},{begin:/([bB]|[bB][rR]|[rR][bB])'/,end:/'/},{begin:/([bB]|[bB][rR]|[rR][bB])"/,end:/"/},{begin:/([fF][rR]|[rR][fF]|[fF])'/,end:/'/,contains:[e.BACKSLASH_ESCAPE,g,u]},{begin:/([fF][rR]|[rR][fF]|[fF])"/,end:/"/,contains:[e.BACKSLASH_ESCAPE,g,u]},e.APOS_STRING_MODE,e.QUOTE_STRING_MODE]},r="[0-9](_?[0-9])*",d=`(\\b(${r}))?\\.(${r})|\\b(${r})\\.`,c=`\\b|${a.join("|")}`,i={className:"number",relevance:0,variants:[{begin:`(\\b(${r})|(${d}))[eE][+-]?(${r})[jJ]?(?=${c})`},{begin:`(${d})[jJ]?`},{begin:`\\b([1-9](_?[0-9])*|0+(_?0)*)[lLjJ]?(?=${c})`},{begin:`\\b0[bB](_?[01])+[lL]?(?=${c})`},{begin:`\\b0[oO](_?[0-7])+[lL]?(?=${c})`},{begin:`\\b0[xX](_?[0-9a-fA-F])+[lL]?(?=${c})`},{begin:`\\b(${r})[jJ](?=${c})`}]},m={className:"comment",begin:t.lookahead(/# type:/),end:/$/,keywords:o,contains:[{begin:/# type:/},{begin:/#/,end:/\b\B/,endsWithParent:!0}]},b={className:"params",variants:[{className:"",begin:/\(\s*\)/,skip:!0},{begin:/\(/,end:/\)/,excludeBegin:!0,excludeEnd:!0,keywords:o,contains:["self",s,i,y,e.HASH_COMMENT_MODE]}]};return u.contains=[y,i,s],{name:"Python",aliases:["py","gyp","ipython"],unicodeRegex:!0,keywords:o,illegal:/(<\/|\?)|=>/,contains:[s,i,{scope:"variable.language",match:/\bself\b/},{beginKeywords:"if",relevance:0},{match:/\bor\b/,scope:"keyword"},y,m,e.HASH_COMMENT_MODE,{match:[/\bdef/,/\s+/,n],scope:{1:"keyword",3:"title.function"},contains:[b]},{variants:[{match:[/\bclass/,/\s+/,n,/\s*/,/\(\s*/,n,/\s*\)/]},{match:[/\bclass/,/\s+/,n]}],scope:{1:"keyword",3:"title.class",6:"title.class.inherited"}},{className:"meta",begin:/^[\t ]*@/,end:/(?=#)|$/,contains:[i,b,y]}]}}const oe={name:"python",register:Q};function J(e){const t="true false yes no null",n="[\\w#;/?:@&=+$,.~*'()[\\]]+",a={className:"attr",variants:[{begin:/[\w*@][\w*@ :()\./-]*:(?=[ \t]|$)/},{begin:/"[\w*@][\w*@ :()\./-]*":(?=[ \t]|$)/},{begin:/'[\w*@][\w*@ :()\./-]*':(?=[ \t]|$)/}]},p={className:"template-variable",variants:[{begin:/\{\{/,end:/\}\}/},{begin:/%\{/,end:/\}/}]},f={className:"string",relevance:0,begin:/'/,end:/'/,contains:[{match:/''/,scope:"char.escape",relevance:0}]},l={className:"string",relevance:0,variants:[{begin:/"/,end:/"/},{begin:/\S+/}],contains:[e.BACKSLASH_ESCAPE,p]},o=e.inherit(l,{variants:[{begin:/'/,end:/'/,contains:[{begin:/''/,relevance:0}]},{begin:/"/,end:/"/},{begin:/[^\s,{}[\]]+/}]}),r={className:"number",begin:"\\b"+"[0-9]{4}(-[0-9][0-9]){0,2}"+"([Tt \\t][0-9][0-9]?(:[0-9][0-9]){2})?"+"(\\.[0-9]*)?"+"([ \\t])*(Z|[-+][0-9][0-9]?(:[0-9][0-9])?)?"+"\\b"},d={end:",",endsWithParent:!0,excludeEnd:!0,keywords:t,relevance:0},c={begin:/\{/,end:/\}/,contains:[d],illegal:"\\n",relevance:0},i={begin:"\\[",end:"\\]",contains:[d],illegal:"\\n",relevance:0},m=[a,{className:"meta",begin:"^---\\s*$",relevance:10},{className:"string",begin:"[\\|>]([1-9]?[+-])?[ ]*\\n( +)[^ ][^\\n]*\\n(\\2[^\\n]+\\n?)*"},{begin:"<%[%=-]?",end:"[%-]?%>",subLanguage:"ruby",excludeBegin:!0,excludeEnd:!0,relevance:0},{className:"type",begin:"!\\w+!"+n},{className:"type",begin:"!<"+n+">"},{className:"type",begin:"!"+n},{className:"type",begin:"!!"+n},{className:"meta",begin:"&"+e.UNDERSCORE_IDENT_RE+"$"},{className:"meta",begin:"\\*"+e.UNDERSCORE_IDENT_RE+"$"},{className:"bullet",begin:"-(?=[ ]|$)",relevance:0},e.HASH_COMMENT_MODE,{beginKeywords:t,keywords:{literal:t}},r,{className:"number",begin:e.C_NUMBER_RE+"\\b",relevance:0},c,i,f,l],b=[...m];return b.pop(),b.push(o),d.contains=b,{name:"YAML",case_insensitive:!0,aliases:["yml"],contains:m}}const ce={name:"yaml",register:J};function se(e){const t="go get go.tracewayapp.com";switch(e){case"gin":return`${t} && go get go.tracewayapp.com/tracewaygin`;case"chi":return`${t} && go get go.tracewayapp.com/tracewaychi`;case"fiber":return`${t} && go get go.tracewayapp.com/tracewayfiber`;case"fasthttp":return`${t} && go get go.tracewayapp.com/tracewayfasthttp`;case"stdlib":return`${t} && go get go.tracewayapp.com/tracewayhttp`;case"react":return"npm install @tracewayapp/react";case"svelte":return"npm install @tracewayapp/svelte";case"vuejs":return"npm install @tracewayapp/vue";case"nextjs":return"npm install @tracewayapp/react";case"nestjs":return"npm install @tracewayapp/nest";case"express":return"npm install @tracewayapp/express";case"remix":return"npm install @tracewayapp/remix";case"jquery":return"npm install @tracewayapp/jquery";case"react-native":return"npm install @tracewayapp/react-native";case"hono":return"";case"symfony":return"composer require traceway/opentelemetry-symfony open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"laravel":return"composer require keepsuit/laravel-opentelemetry open-telemetry/exporter-otlp php-http/guzzle7-adapter";case"django":return"pip install opentelemetry-distro opentelemetry-exporter-otlp opentelemetry-instrumentation-django && opentelemetry-bootstrap -a install";case"cloudflare":return"";case"opentelemetry":return"";case"flutter":return"flutter pub add traceway";case"android":return'implementation("com.tracewayapp:traceway:1.0.0")';default:return t}}function ie(e,t,n){const a=t?`${t}@${n}/api/report`:`YOUR_TOKEN@${n}/api/report`;switch(e){case"gin":return`package main

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
}`}}function pe(e){return e==="symfony"?`<?php
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
throw RuntimeException("Test error from Traceway integration")`:e&&U(e)?`// Trigger a test error
throw new Error("Test error from Traceway integration");`:`r.GET("/testing", func(c *gin.Context) {
    panic("Test error from Traceway integration")
})`}function le(e){if(e==="symfony"||e==="laravel"||e==="django")return"";if(e==="flutter")return`import 'package:traceway/traceway.dart';

TracewayClient.instance?.captureException(
  Exception('Test error'),
  StackTrace.current,
);`;if(e==="android")return`import com.tracewayapp.traceway.Traceway

try {
    riskyOperation()
} catch (e: Throwable) {
    Traceway.captureException(e)
}`;if(e&&U(e))switch(e){case"react":return`import { useTraceway } from "@tracewayapp/react";

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
captureException(new Error("Test error"));`;default:return`import { captureException } from "@tracewayapp/${ee(e)}";

captureException(new Error("Test error"));`}return`r.GET("/testing", func(c *gin.Context) {
    c.AbortWithError(500, traceway.NewStackTraceErrorf("testing"))
})`}function ee(e){switch(e){case"react":return"react";case"svelte":return"svelte";case"vuejs":return"vue";case"nextjs":return"next";case"nestjs":return"nest";case"express":return"express";case"remix":return"remix";case"jquery":return"jquery";case"react-native":return"react-native";default:return"react"}}function ue(e){return{gin:"Gin",fiber:"Fiber",chi:"Chi",fasthttp:"FastHTTP",stdlib:"Standard Library (net/http)",custom:"Custom Integration",react:"React",svelte:"Svelte",vuejs:"Vue.js",nextjs:"Next.js",nestjs:"NestJS",express:"Express",remix:"Remix",jquery:"jQuery","react-native":"React Native",hono:"Hono",cloudflare:"Cloudflare",opentelemetry:"OpenTelemetry",symfony:"Symfony",laravel:"Laravel",django:"Django",flutter:"Flutter",android:"Android"}[e]||e}function de(e){return e==="symfony"||e==="laravel"?"php":e==="django"?"python":e==="opentelemetry"?"go":e==="hono"||e==="cloudflare"||e==="flutter"||e==="android"||U(e)?"javascript":"go"}export{ie as a,ne as b,oe as c,se as d,pe as e,le as f,ue as g,de as h,ae as j,re as p,ce as y};
