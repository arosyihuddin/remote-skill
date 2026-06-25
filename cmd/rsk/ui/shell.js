/* --- shell --- */
var cmdHistory=[]
var histIdx=-1
var user=''
var pwd=''
var dirDisp='~'
var gitBranch=''
var gitStatus=''

function fetchPromptInfo(){
  return api('POST','/exec',{cmd:['sh','-c',`whoami 2>/dev/null;echo "---";pwd 2>/dev/null;echo "---";git symbolic-ref --short HEAD 2>/dev/null||true;echo "---";git status --porcelain 2>/dev/null|head -20||true`],timeout_sec:5}).then(r=>{
    if(!r.stdout)return
    const parts=r.stdout.split('---\n')
    user=(parts[0]||'').trim()||'user'
    const home='/home/'+user
    pwd=home
    dirDisp='~'
    gitBranch=(parts[2]||'').trim()
    var gs=parts[3]||''
    gitStatus=gs.trim()?calcGitStatus(gs):''
    renderPrompt($('inpPrompt'))
  }).catch(()=>{})
}

function calcGitStatus(porcelain){
  var s={m:0,a:0,d:0,'?':0}
  porcelain.split('\n').forEach(l=>{
    var c=l.charAt(0)
    if(c==='M'||c===' '&&l.charAt(1)==='M')s.m++
    else if(c==='A')s.a++
    else if(c==='D')s.d++
    else if(c==='?')s['?']++
  })
  var out=[]
  if(s.a)out.push('+'+s.a)
  if(s.m)out.push('!'+s.m)
  if(s.d)out.push('x'+s.d)
  if(s['?'])out.push('?'+s['?'])
  return out.join(' ')
}

function renderPrompt(container){
  var h='<span class="prompt-seg usr">'+escapeHtml(user)+'</span>'
  h+='<span class="prompt-seg dir">'+escapeHtml(dirDisp)+'</span>'
  if(gitBranch) h+='<span class="prompt-seg git">'+escapeHtml(gitBranch)+(gitStatus?' '+escapeHtml(gitStatus):'')+'</span>'
  h+='<span class="prompt-seg time" id="promptTime">'+now()+'</span>'
  container.innerHTML=h
}

function now(){
  var d=new Date()
  return String(d.getHours()).padStart(2,'0')+':'+String(d.getMinutes()).padStart(2,'0')
}

setInterval(function(){var e=$('promptTime');if(e)e.textContent=now()},10000)

var autoItems=[],autoIdx=-1,autoWord='',autoWordStart=0,autoIsFirst=false

$('shellCmd').onkeydown=function(e){
  const dd=$('autoDropdown')
  if(e.key==='Tab'){
    e.preventDefault()
    if(dd.style.display==='block'){
      e.shiftKey?autoIdx--:autoIdx++
      if(autoIdx<0)autoIdx=autoItems.length-1
      if(autoIdx>=autoItems.length)autoIdx=0
      showAutoDropdown()
      return
    }
    doAutoComplete(this)
    return
  }
  if(e.key==='Escape'){
    if(dd.style.display==='block'){hideAutoDropdown();e.preventDefault()}
    return
  }
  if(e.key==='Enter'){
    if(dd.style.display==='block'&&autoIdx>=0){
      e.preventDefault();applyAutoComplete(this);return
    }
    const cmd=this.value.trim()
    if(!cmd)return
    this.value=''
    if(cmd==='clear'){$('termBody').querySelectorAll(':scope > :not(.term-input-line)').forEach(e=>e.remove());$('shellCmd').focus();return}
    cmdHistory.push(cmd)
    histIdx=cmdHistory.length
    const body=$('termBody'),inputLine=body.querySelector('.term-input-line')
    const[prLine,cmdLine]=renderCmdLine(cmd)
    body.insertBefore(prLine,inputLine)
    body.insertBefore(cmdLine,inputLine)
    const runLine=termLine('<span style="color:#666">running...</span>',true)
    body.insertBefore(runLine,inputLine)
    inputLine.scrollIntoView({block:'end'})
    if(/^cd(\s|$)/.test(cmd)){
      body.removeChild(runLine)
      handleCD(cmd)
      return
    }
    const realCmd=pwd?'cd '+quote(pwd)+' && '+cmd:cmd
    api('POST','/exec',{cmd:['sh','-c',realCmd],timeout_sec:30}).then(r=>{
      body.removeChild(runLine)
      if(/^ls(\s|$)/.test(cmd)&&r.stdout){
        body.insertBefore(lsGrid(r.stdout.trim()),inputLine)
      }else{
        var s=''
        if(r.stdout)s+=escapeHtml(r.stdout)
        if(r.stderr)s+='\n<span class="error">'+escapeHtml(r.stderr)+'</span>'
        if(r.exit_code!==0)s+='\n<span class="error">[exit '+r.exit_code+']</span>'
        if(s) body.insertBefore(termLine(s,true),inputLine)
      }
      inputLine.scrollIntoView({block:'end'})
      $('shellCmd').focus()
    }).catch(e=>{
      body.removeChild(runLine)
      body.insertBefore(termLine('<span class="error">Error: '+escapeHtml(e.message)+'</span>',true),inputLine)
      inputLine.scrollIntoView({block:'end'})
      $('shellCmd').focus()
    })
  }else if(e.key==='ArrowUp'){
    if(histIdx>0){histIdx--;this.value=cmdHistory[histIdx]}
    e.preventDefault()
  }else if(e.key==='ArrowDown'){
    if(histIdx<cmdHistory.length-1){histIdx++;this.value=cmdHistory[histIdx]}
    else{histIdx=cmdHistory.length;this.value=''}
    e.preventDefault()
  }else if(e.key==='l'&&(e.ctrlKey||e.metaKey)){
    e.preventDefault()
    $('termBody').querySelectorAll(':scope > :not(.term-input-line)').forEach(e=>e.remove())
    $('shellCmd').focus()
  }
}
$('shellOutput').onclick=function(){$('shellCmd').focus()}
$('shellCmd').oninput=function(){hideAutoDropdown()}

function doAutoComplete(input){
  const val=input.value
  const cursor=input.selectionStart
  const before=val.slice(0,cursor)
  autoWordStart=before.lastIndexOf(' ')+1
  autoWord=val.slice(autoWordStart,cursor)
  autoIsFirst=autoWordStart===0
  if(autoIsFirst){
    const cmds=['cd','ls','clear','pwd','echo','cat','cp','mv','rm','mkdir','touch','git','go','head','tail','less','grep']
    autoItems=cmds.concat(cmdHistory).filter(c=>c.startsWith(autoWord)).filter((v,i,a)=>a.indexOf(v)===i)
    if(autoItems.length===1){input.value=val.slice(0,autoWordStart)+autoItems[0]+val.slice(cursor);input.setSelectionRange(autoWordStart+autoItems[0].length,autoWordStart+autoItems[0].length);hideAutoDropdown();return}
    autoIdx=-1;showAutoDropdown()
  }else{
    const prefix=val.slice(0,autoWordStart)
    const lastSlash=autoWord.lastIndexOf('/')
    const dirPart=lastSlash<0?'':autoWord.slice(0,lastSlash+1)
    const filePrefix=lastSlash<0?autoWord:autoWord.slice(lastSlash+1)
    const baseDir=dirPart.startsWith('/')?dirPart:(pwd?pwd+'/':'')+dirPart
    api('POST','/ls',{path:baseDir||'/'}).then(r=>{
      if(!r.entries)return
      autoItems=r.entries.filter(e=>e.name.startsWith(filePrefix)).map(e=>e.name+(e.is_dir?'/':''))
      if(autoItems.length===1){input.value=prefix+dirPart+autoItems[0]+val.slice(cursor);input.setSelectionRange((prefix+dirPart+autoItems[0]).length,(prefix+dirPart+autoItems[0]).length);hideAutoDropdown();return}
      autoIdx=-1;showAutoDropdown()
    }).catch(()=>{})
  }
}

function showAutoDropdown(){
  const dd=$('autoDropdown')
  if(!autoItems.length){dd.style.display='none';return}
  dd.innerHTML=autoItems.map((n,i)=>{
    const cl='auto-item'+(i===autoIdx?' active':'')
    return '<div class="'+cl+'">'+fileIcon(n)+escapeHtml(n)+'</div>'
  }).join('')
  const rect=$('shellCmd').getBoundingClientRect()
  dd.style.position='fixed'
  dd.style.left=rect.left+'px'
  dd.style.width=rect.width+'px'
  dd.style.bottom=(window.innerHeight-rect.top)+'px'
  dd.style.display='block'
  if(autoIdx>=0) dd.querySelector('.active')?.scrollIntoView({block:'nearest'})
}

function hideAutoDropdown(){
  $('autoDropdown').style.display='none'
  autoItems=[];autoIdx=-1
}

function applyAutoComplete(input){
  if(autoIdx<0||autoIdx>=autoItems.length)return
  const val=input.value
  const sel=autoItems[autoIdx]
  input.value=val.slice(0,autoWordStart)+sel+val.slice(input.selectionStart)
  const pos=autoWordStart+sel.length
  input.setSelectionRange(pos,pos)
  hideAutoDropdown()
  input.focus()
}

function renderCmdLine(cmd){
  const promptLine=document.createElement('div');promptLine.className='term-line'
  promptLine.innerHTML='<div class="prompt">'+renderPromptStr()+'</div>'
  const cmdLine=document.createElement('div');cmdLine.className='term-line'
  cmdLine.innerHTML='<span style="color:var(--green);font-weight:600">▶</span> <span class="cmd">'+escapeHtml(cmd)+'</span>'
  return[promptLine,cmdLine]
}

function renderPromptStr(){
  var h='<span class="prompt-seg usr">'+escapeHtml(user)+'</span>'
  h+='<span class="prompt-seg dir">'+escapeHtml(dirDisp)+'</span>'
  if(gitBranch) h+='<span class="prompt-seg git">'+escapeHtml(gitBranch)+(gitStatus?' '+escapeHtml(gitStatus):'')+'</span>'
  h+='<span class="prompt-seg time">'+now()+'</span>'
  return h
}

function handleCD(cmd){
  var target=(cmd.match(/^cd\s+(.+)/)||[])[1]||''
  if(!target||target==='~'){target='~'}
  target=target.replace(/^~/, '/home/'+user)
  api('POST','/exec',{cmd:['sh','-c','cd '+quote(pwd)+' && cd '+quote(target)+' && pwd && echo "---" && (git symbolic-ref --short HEAD 2>/dev/null||echo "") && echo "---" && (git status --porcelain 2>/dev/null|head -20||true)'],timeout_sec:5}).then(r=>{
    if(r.stdout){
      var parts=r.stdout.split('---\n')
      pwd=(parts[0]||'').trim()
      dirDisp=pwd.replace(RegExp('^'+escapeRegex('/home/'+user)),'~')
      gitBranch=(parts[1]||'').trim()
      var gs=(parts[2]||'').trim()
      gitStatus=gs?calcGitStatus(gs):''
      renderPrompt($('inpPrompt'))
    }
  }).catch(()=>{})
}

function quote(s){return "'"+s.replace(/'/g,"'\\''")+"'"}

function termLine(html,isOutput){const d=document.createElement('div');d.className='term-line'+(isOutput?' output':'');d.innerHTML=html;return d}

// init prompt — render before async fetch
renderPrompt($('inpPrompt'))
fetchPromptInfo()
