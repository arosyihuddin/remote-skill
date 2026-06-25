const BASE=''
var deviceId=''
var deviceList=[]
var authenticated=false
var ws=null,screenData={width:0,height:0}
var speed=2,mode='move'
var tp={touching:false,lastX:0,lastY:0,startX:0,startY:0,startTime:0,moved:false}
var scMods=[]

function $(id){return document.getElementById(id)}

function api(method,path,body,dev){
  const opts={method,headers:{'Content-Type':'application/json'}}
  if(body!==undefined) opts.body=JSON.stringify(body)
  let url=BASE+path
  const d=dev!==undefined?dev:deviceId
  if(d) url+=(url.includes('?')?'&':'?')+'device='+encodeURIComponent(d)
  return fetch(url,opts).then(r=>{if(!r.ok)return r.text().then(t=>{throw new Error(t)});return r.json()})
}

/* --- auth --- */
function checkAuth(){
  return fetch(BASE+'/devices').then(r=>{
    if(!r.ok) throw new Error('unauthorized')
    return r.json()
  }).then(data=>{
    authenticated=true
    $('loginOverlay').style.display='none'
    $('logoutBtn').style.display='flex'
    onDevices(data)
    fetchPromptInfo()
  }).catch(()=>{
    authenticated=false
    $('loginOverlay').style.display='flex'
    $('logoutBtn').style.display='none'
  })
}

$('loginBtn').onclick=function(){
  const token=$('loginToken').value.trim()
  if(!token)return
  $('loginError').textContent=''
  fetch(BASE+'/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({token})})
  .then(r=>{if(!r.ok)throw new Error();return r.json()})
  .then(()=>{checkAuth()})
  .catch(()=>{$('loginError').textContent='Invalid password';$('loginToken').value='';$('loginToken').focus()})
}
$('loginToken').onkeydown=e=>{if(e.key==='Enter')$('loginBtn').click()}
$('loginToken').oninput=()=>{$('loginError').textContent=''}
$('pwEye').onclick=function(){
  const inp=$('loginToken')
  const e=this.querySelector('.eye-open'),o=this.querySelector('.eye-off')
  if(inp.type==='password'){inp.type='text';e.style.display='none';o.style.display=''}
  else{inp.type='password';e.style.display='';o.style.display='none'}
}
$('logoutBtn').onclick=async function(){try{await fetch(BASE+'/logout',{method:'POST'})}catch(_){}location.reload()}

/* --- device selector --- */
function updateDeviceSelector(){
  const sel=$('deviceSelector')
  const prev=sel.value
  sel.innerHTML=''
  if(!deviceList.length){
    sel.innerHTML='<option value="">— no devices —</option>'
    deviceId=''
    return
  }
  var found=false
  deviceList.forEach(d=>{
    const label=(d.hostname||d.id)+(d.os?' • '+d.os:'')
    sel.innerHTML+='<option value="'+d.id+'">● '+label+'</option>'
    if(d.id===prev) found=true
  })
  if(found) sel.value=prev
  else if(deviceList.length>0) sel.value=deviceList[0].id
  deviceId=sel.value
}
$('deviceSelector').onchange=function(){deviceId=this.value;stopLive();onDeviceChange()}

function onDevices(data){
  const prevId=deviceId
  deviceList=data.devices||[]
  updateDeviceSelector()
  if(deviceId!==prevId) onDeviceChange()
}

function onDeviceChange(){updateDashboard()}

/* --- tab switching --- */
document.querySelectorAll('.nav-item').forEach(btn=>{
  btn.onclick=function(){
    document.querySelectorAll('.nav-item').forEach(b=>b.classList.remove('active'))
    document.querySelectorAll('.main').forEach(m=>m.classList.remove('active'))
    this.classList.add('active')
    const tab=$('tab-'+this.dataset.tab)
    if(tab){
      tab.classList.add('active')
      if(this.dataset.tab==='files'&&!fileHistory.length) listDir(pwd||'/')
    }
  }
})

/* --- dashboard --- */
function updateDashboard(){
  if(!deviceId)return
  api('POST','/exec',{cmd:['sh','-c','echo "$(hostname)|$(uname -srm)|$(uptime -p)|$(basename $SHELL)|$HOME|$XDG_CURRENT_DESKTOP"'],timeout_sec:5}).then(r=>{
    if(r.stdout){
      const p=r.stdout.trim().split('|')
      $('dHostname').textContent=p[0]||'—'
      $('dOS').textContent=p[1]||'—'
      $('dUptime').textContent=p[2]||'—'
      $('dShell').textContent=p[3]||'—'
      $('dHome').textContent=p[4]||'—'
      $('dDesktop').textContent=p[5]||'—'
    }
  }).catch(()=>{})
  if(!$('appsGrid').dataset.loaded){loadApps();$('appsGrid').dataset.loaded='1'}
  loadActiveApps()
}

/* --- remote: live screen --- */
function screenURL(){
  var url=(location.protocol==='https:'?'wss:':'ws:')+'//'+location.host+'/screen.ws'
  if(deviceId) url+='?device='+encodeURIComponent(deviceId)
  return url
}
$('liveBtn').onclick=function(){ws?stopLive():startLive()}
function startLive(){
  try{ws=new WebSocket(screenURL())}catch(e){$('screenPlaceholder').textContent='WS error: '+e.message;return}
  ws.binaryType='blob'
  ws.onopen=()=>{$('liveBtn').innerHTML='<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><rect x="4" y="4" width="16" height="16" rx="2"/></svg> Stop Live';$('screenPlaceholder').style.display='none';$('screenCanvas').style.display='block'}
  ws.onmessage=async e=>{try{
    const c=$('screenCanvas');const b=await createImageBitmap(e.data)
    screenData.width=b.width;screenData.height=b.height
    c.width=b.width;c.height=b.height
    c.getContext('2d').drawImage(b,0,0);b.close()
  }catch(_){}}
  ws.onclose=()=>cleanupLive()
  ws.onerror=()=>cleanupLive()
}
function stopLive(){if(ws){ws.close();ws=null}cleanupLive()}
function cleanupLive(){ws=null;$('liveBtn').innerHTML='<svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg> Start Live'}

/* remote: screenshot */
$('shotBtn').onclick=function(){
  const c=$('screenCanvas');const p=$('screenPlaceholder')
  api('POST','/screenshot',{}).then(r=>{
    if(r.base64){
      const img=new Image()
      img.onload=function(){c.width=img.width;c.height=img.height;c.getContext('2d').drawImage(img,0,0);p.style.display='none';c.style.display='block';img.remove()}
      img.src='data:image/png;base64,'+r.base64
    }else{p.style.display='block';p.textContent='(no image data)'}
  }).catch(e=>{p.style.display='block';p.textContent='Error: '+e.message})
}

/* remote: canvas click -> mouse */
$('screenWrap').onclick=function(e){
  const c=$('screenCanvas')
  if(!c||c.style.display==='none'||!screenData.width)return
  const r=c.getBoundingClientRect()
  const sx=screenData.width/r.width,sy=screenData.height/r.height
  const x=Math.round((e.clientX-r.left)*sx),y=Math.round((e.clientY-r.top)*sy)
  $('clickPos').textContent=x+','+y
  $('mouseX').value=x;$('mouseY').value=y
  api('POST','/mouse',{x,y}).catch(()=>{})
}

/* remote: mouse */
$('mouseGoBtn').onclick=function(){
  const x=parseInt($('mouseX').value)||0,y=parseInt($('mouseY').value)||0
  api('POST','/mouse',{x,y}).then(()=>{$('clickPos').textContent=x+','+y+' moved'}).catch(e=>showToast('Mouse: '+e.message,'error'))
}
$('clickLeft').onclick=()=>api('POST','/click',{button:'left'}).catch(e=>showToast(e.message,'error'))
$('clickRight').onclick=()=>api('POST','/click',{button:'right'}).catch(e=>showToast(e.message,'error'))
$('clickMiddle').onclick=()=>api('POST','/click',{button:'middle'}).catch(e=>showToast(e.message,'error'))
$('clickDouble').onclick=()=>api('POST','/click',{button:'left',double:true}).catch(e=>showToast(e.message,'error'))
$('mouseX').onkeydown=e=>{if(e.key==='Enter')$('mouseGoBtn').click()}
$('mouseY').onkeydown=e=>{if(e.key==='Enter')$('mouseGoBtn').click()}

/* remote: touchpad */
const touchpad=$('touchpad'),cursor=$('touchpadCursor'),slider=$('speedSlider'),speedLabel=$('speedLabel')
slider.oninput=()=>{speed=parseFloat(slider.value);speedLabel.textContent=speed.toFixed(1)+'×'}
$('modeToggle').onclick=function(){
  mode=mode==='move'?'scroll':'move'
  this.textContent=mode==='move'?'Move':'Scroll'
}
function tpDown(px,py){tp.touching=true;tp.moved=false;tp.lastX=px;tp.lastY=py;tp.startX=px;tp.startY=py;tp.startTime=Date.now();touchpad.classList.add('active');cursor.style.left=px+'px';cursor.style.top=py+'px'}
function tpMove(px,py){
  if(!tp.touching)return
  const dx=(px-tp.lastX)*speed,dy=(py-tp.lastY)*speed
  tp.lastX=px;tp.lastY=py
  if(Math.abs(px-tp.startX)>4||Math.abs(py-tp.startY)>4) tp.moved=true
  if(Math.abs(dx)>=1||Math.abs(dy)>=1){cursor.style.left=px+'px';cursor.style.top=py+'px';api('POST','/mouse',{x:Math.round(dx),y:Math.round(dy),relative:true}).catch(()=>{})}
}
function tpUp(e){
  if(!tp.touching)return
  if(!tp.moved&&(Date.now()-tp.startTime)<500) api('POST','/click',{button:'left'}).catch(()=>{})
  tp.touching=false;touchpad.classList.remove('active')
}
touchpad.onmousedown=e=>{tpDown(e.clientX-touchpad.getBoundingClientRect().left,e.clientY-touchpad.getBoundingClientRect().top);e.preventDefault()}
document.onmousemove=e=>{if(tp.touching)tpMove(e.clientX-touchpad.getBoundingClientRect().left,e.clientY-touchpad.getBoundingClientRect().top)}
document.onmouseup=e=>{if(tp.touching)tpUp(e)}
touchpad.ontouchstart=e=>{const r=touchpad.getBoundingClientRect();tpDown(e.touches[0].clientX-r.left,e.touches[0].clientY-r.top);e.preventDefault()}
touchpad.ontouchmove=e=>{const r=touchpad.getBoundingClientRect();tpMove(e.touches[0].clientX-r.left,e.touches[0].clientY-r.top);e.preventDefault()}
touchpad.ontouchend=e=>{tpUp(e)}

/* remote: keyboard */
$('typeBtn').onclick=function(){const t=$('typeText').value;if(t)api('POST','/type',{text:t}).catch(e=>showToast(e.message,'error'))}
$('typeText').onkeydown=e=>{if(e.key==='Enter')$('typeBtn').click()}
document.querySelectorAll('#tab-remote button[data-combo]').forEach(btn=>{btn.onclick=function(){api('POST','/key',{combo:this.dataset.combo}).catch(e=>showToast(e.message,'error'))}})

/* remote: clipboard */
$('clipRead').onclick=function(){api('POST','/clipboard/read',{}).then(r=>{$('clipOutput').textContent=r.content||'(empty)'}).catch(e=>{$('clipOutput').textContent='Error: '+e.message})}
$('clipWrite').onclick=function(){const t=$('clipText').value;if(t)api('POST','/clipboard/write',{content:t}).then(()=>{$('clipOutput').textContent='written'}).catch(e=>{$('clipOutput').textContent='Error: '+e.message})}

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

var codeIcon='<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M14.4473 3.02637C14.847 3.13536 15.0826 3.54766 14.9736 3.94728L10.4736 20.4473C10.3646 20.8469 9.95228 21.0825 9.55266 20.9735C9.15304 20.8645 8.91744 20.4522 9.02643 20.0526L13.5264 3.55261C13.6354 3.15299 14.0477 2.91738 14.4473 3.02637ZM16.7197 6.21961C17.0126 5.92672 17.4874 5.92672 17.7803 6.21961L23.0303 11.4696C23.3232 11.7625 23.3232 12.2374 23.0303 12.5303L17.7803 17.7803C17.4874 18.0732 17.0126 18.0732 16.7197 17.7803C16.4268 17.4874 16.4268 17.0125 16.7197 16.7196L21.4393 11.9999L16.7197 7.28027C16.4268 6.98738 16.4268 6.51251 16.7197 6.21961ZM7.28033 6.21961C7.57322 6.51251 7.57322 6.98738 7.28033 7.28027L2.56066 11.9999L7.28033 16.7196C7.57322 17.0125 7.57322 17.4874 7.28033 17.7803C6.98744 18.0732 6.51256 18.0732 6.21967 17.7803L0.96967 12.5303C0.676777 12.2374 0.676777 11.7625 0.96967 11.4696L6.21967 6.21961C6.51256 5.92672 6.98744 5.92672 7.28033 6.21961Z"/></svg>'
var docIcon='<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M5.625 1.5C4.58947 1.5 3.75 2.33947 3.75 3.375V20.625C3.75 21.6605 4.58947 22.5 5.625 22.5H18.375C19.4105 22.5 20.25 21.6605 20.25 20.625V12.75C20.25 10.6789 18.5711 9 16.5 9H14.625C13.5895 9 12.75 8.16053 12.75 7.125V5.25C12.75 3.17893 11.0711 1.5 9 1.5H5.625ZM7.5 15C7.5 14.5858 7.83579 14.25 8.25 14.25H15.75C16.1642 14.25 16.5 14.5858 16.5 15C16.5 15.4142 16.1642 15.75 15.75 15.75H8.25C7.83579 15.75 7.5 15.4142 7.5 15ZM8.25 17.25C7.83579 17.25 7.5 17.5858 7.5 18C7.5 18.4142 7.83579 18.75 8.25 18.75H12C12.4142 18.75 12.75 18.4142 12.75 18C12.75 17.5858 12.4142 17.25 12 17.25H8.25Z"/><path d="M12.9712 1.8159C13.768 2.73648 14.25 3.93695 14.25 5.25V7.125C14.25 7.33211 14.4179 7.5 14.625 7.5H16.5C17.8131 7.5 19.0135 7.98204 19.9341 8.77881C19.0462 5.37988 16.3701 2.70377 12.9712 1.8159Z"/></svg>'
var imgIcon='<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M1.5 6C1.5 4.75736 2.50736 3.75 3.75 3.75H20.25C21.4926 3.75 22.5 4.75736 22.5 6V18C22.5 19.2426 21.4926 20.25 20.25 20.25H3.75C2.50736 20.25 1.5 19.2426 1.5 18V6ZM3 16.0607V18C3 18.4142 3.33579 18.75 3.75 18.75H20.25C20.6642 18.75 21 18.4142 21 18V16.0607L18.3107 13.3713C17.7249 12.7855 16.7751 12.7855 16.1893 13.3713L15.3107 14.25L16.2803 15.2197C16.5732 15.5126 16.5732 15.9874 16.2803 16.2803C15.9874 16.5732 15.5126 16.5732 15.2197 16.2803L10.0607 11.1213C9.47487 10.5355 8.52513 10.5355 7.93934 11.1213L3 16.0607ZM13.125 8.25C13.125 7.62868 13.6287 7.125 14.25 7.125C14.8713 7.125 15.375 7.62868 15.375 8.25C15.375 8.87132 14.8713 9.375 14.25 9.375C13.6287 9.375 13.125 8.87132 13.125 8.25Z"/></svg>'
var audioIcon='<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M13.5 4.06c0-1.336-1.616-2.005-2.56-1.06l-4.5 4.5H4.508c-1.141 0-2.318.664-2.66 1.905A9.76 9.76 0 001.5 12c0 .898.121 1.768.35 2.595.341 1.24 1.518 1.905 2.659 1.905h1.93l4.5 4.5c.945.945 2.561.276 2.561-1.06V4.06zM18.584 5.106a.75.75 0 011.06 0c3.808 3.807 3.808 9.98 0 13.788a.75.75 0 11-1.06-1.06 8.25 8.25 0 000-11.668.75.75 0 010-1.06zM15.932 7.757a.75.75 0 011.061 0 6 6 0 010 8.486.75.75 0 01-1.06-1.061 4.5 4.5 0 000-6.364.75.75 0 010-1.06z"/></svg>'
var videoIcon='<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M4.5 4.5a3 3 0 00-3 3v9a3 3 0 003 3h8.25a3 3 0 003-3v-9a3 3 0 00-3-3H4.5zM19.94 18.75l-2.69-2.69V7.94l2.69-2.69c.944-.945 2.56-.276 2.56 1.06v11.38c0 1.336-1.616 2.005-2.56 1.06z"/></svg>'

function fileIcon(name){
  if(name.endsWith('/')) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M19.5 21C21.1569 21 22.5 19.6569 22.5 18V13.5C22.5 11.8431 21.1569 10.5 19.5 10.5H4.5C2.84315 10.5 1.5 11.8431 1.5 13.5V18C1.5 19.6569 2.84315 21 4.5 21H19.5Z"/><path d="M1.5 10.1458V6C1.5 4.34315 2.84315 3 4.5 3H9.87868C10.4754 3 11.0477 3.23705 11.4697 3.65901L13.591 5.78033C13.7316 5.92098 13.9224 6 14.1213 6H19.5C21.1569 6 22.5 7.34315 22.5 9V10.1458C21.7039 9.43328 20.6525 9 19.5 9H4.5C3.34747 9 2.29613 9.43328 1.5 10.1458Z"/></svg>'
  var ext=name.split('.').pop().toLowerCase()
  if(['go','py','js','ts','jsx','tsx','rs','rb','java','c','cpp','h','hpp','cs','swift','kt','php','pl','r','dart','lua','hs','clj','scala','ex','exs','html','htm','css','scss','sass','less','vue','svelte','xml'].includes(ext)) return codeIcon
  if(['sh','bash','zsh','fish'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M2.25 6C2.25 4.34315 3.59315 3 5.25 3H18.75C20.4069 3 21.75 4.34315 21.75 6V18C21.75 19.6569 20.4069 21 18.75 21H5.25C3.59315 21 2.25 19.6569 2.25 18V6ZM6.21967 6.96967C6.51256 6.67678 6.98744 6.67678 7.28033 6.96967L9.53033 9.21967C9.82322 9.51256 9.82322 9.98744 9.53033 10.2803L7.28033 12.5303C6.98744 12.8232 6.51256 12.8232 6.21967 12.5303C5.92678 12.2374 5.92678 11.7626 6.21967 11.4697L7.93934 9.75L6.21967 8.03033C5.92678 7.73744 5.92678 7.26256 6.21967 6.96967ZM10.5 11.25C10.0858 11.25 9.75 11.5858 9.75 12C9.75 12.4142 10.0858 12.75 10.5 12.75H13.5C13.9142 12.75 14.25 12.4142 14.25 12C14.25 11.5858 13.9142 11.25 13.5 11.25H10.5Z"/></svg>'
  if(['md','txt','markdown','pdf','doc','docx','xls','xlsx','ppt','pptx','csv','epub','log'].includes(ext)) return docIcon
  if(['png','jpg','jpeg','gif','svg','webp','ico','bmp','tiff','tif','raw'].includes(ext)) return imgIcon
  if(['zip','tar','gz','bz2','xz','7z','rar','iso','deb','rpm'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M3.375 3C2.33947 3 1.5 3.83947 1.5 4.875V5.625C1.5 6.66053 2.33947 7.5 3.375 7.5H20.625C21.6605 7.5 22.5 6.66053 22.5 5.625V4.875C22.5 3.83947 21.6605 3 20.625 3H3.375Z"/><path fill-rule="evenodd" d="M3.08679 9L3.62657 18.1762C3.71984 19.7619 5.03296 21 6.62139 21H17.3783C18.9667 21 20.2799 19.7619 20.3731 18.1762L20.9129 9H3.08679ZM9.24976 12.75C9.24976 12.3358 9.58554 12 9.99976 12H13.9998C14.414 12 14.7498 12.3358 14.7498 12.75C14.7498 13.1642 14.414 13.5 13.9998 13.5H9.99976C9.58554 13.5 9.24976 13.1642 9.24976 12.75Z"/></svg>'
  if(['db','sqlite','sqlite3'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path d="M21 6.375C21 9.06739 16.9706 11.25 12 11.25C7.02944 11.25 3 9.06739 3 6.375C3 3.68261 7.02944 1.5 12 1.5C16.9706 1.5 21 3.68261 21 6.375Z"/><path d="M12 12.75C14.6852 12.75 17.1905 12.1637 19.0784 11.1411C19.7684 10.7673 20.4248 10.3043 20.9747 9.75674C20.9915 9.87831 21 10.0011 21 10.125C21 12.8174 16.9706 15 12 15C7.02944 15 3 12.8174 3 10.125C3 10.0011 3.00853 9.8783 3.02529 9.75674C3.57523 10.3043 4.23162 10.7673 4.92161 11.1411C6.80949 12.1637 9.31481 12.75 12 12.75Z"/></svg>'
  if(['json','yaml','yml','toml','ini','cfg','conf','ttf','otf','woff','woff2','eot'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M11.0779 2.25C10.1613 2.25 9.37909 2.91265 9.22841 3.81675L9.04974 4.88873C9.02959 5.00964 8.93542 5.1498 8.75311 5.23747C8.40905 5.40292 8.07967 5.5938 7.7674 5.8076C7.60091 5.92159 7.43259 5.9332 7.31769 5.89015L6.29851 5.50833C5.44019 5.18678 4.4752 5.53289 4.01692 6.32666L3.09493 7.92358C2.63665 8.71736 2.8194 9.72611 3.52704 10.3087L4.36756 11.0006C4.46219 11.0785 4.53629 11.2298 4.52119 11.4307C4.50706 11.6188 4.49988 11.8086 4.49988 12C4.49988 12.1915 4.50707 12.3814 4.52121 12.5695C4.53632 12.7704 4.46221 12.9217 4.36758 12.9996L3.52704 13.6916C2.8194 14.2741 2.63665 15.2829 3.09493 16.0767L4.01692 17.6736C4.4752 18.4674 5.44019 18.8135 6.29851 18.4919L7.31791 18.11C7.43281 18.067 7.60113 18.0786 7.76761 18.1925C8.07982 18.4063 8.40913 18.5971 8.75311 18.7625C8.93542 18.8502 9.02959 18.9904 9.04974 19.1113L9.22841 20.1832C9.37909 21.0874 10.1613 21.75 11.0779 21.75H12.9219C13.8384 21.75 14.6207 21.0874 14.7713 20.1832L14.95 19.1113C14.9702 18.9904 15.0643 18.8502 15.2466 18.7625C15.5907 18.5971 15.9201 18.4062 16.2324 18.1924C16.3988 18.0784 16.5672 18.0668 16.6821 18.1098L17.7012 18.4917C18.5596 18.8132 19.5246 18.4671 19.9828 17.6733L20.9048 16.0764C21.3631 15.2826 21.1804 14.2739 20.4727 13.6913L19.6322 12.9994C19.5376 12.9215 19.4635 12.7702 19.4786 12.5693C19.4927 12.3812 19.4999 12.1914 19.4999 12C19.4999 11.8085 19.4927 11.6186 19.4785 11.4305C19.4634 11.2296 19.5375 11.0783 19.6322 11.0004L20.4727 10.3084C21.1804 9.72587 21.3631 8.71711 20.9048 7.92334L19.9828 6.32642C19.5246 5.53264 18.5596 5.18654 17.7012 5.50809L16.6818 5.89C16.5669 5.93304 16.3986 5.92144 16.2321 5.80746C15.9199 5.59371 15.5906 5.40289 15.2466 5.23747C15.0643 5.1498 14.9702 5.00964 14.95 4.88873L14.7713 3.81675C14.6207 2.91265 13.8384 2.25 12.9219 2.25H11.0779ZM12 15.75a3.75 3.75 0 1 0 0-7.5 3.75 3.75 0 0 0 0 7.5Z" clip-rule="evenodd"/></svg>'
  if(['mp3','wav','flac','aac','ogg','m4a','wma'].includes(ext)) return audioIcon
  if(['mp4','avi','mkv','mov','wmv','webm','flv'].includes(ext)) return videoIcon
  if(ext===''||['exe','bin','AppImage'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>'
  return docIcon
}

function lsGrid(text){
  const items=text.split(/\s+/).filter(Boolean)
  const d=document.createElement('div');d.className='ls-grid'
  items.forEach(n=>{
    const s=document.createElement('span');s.className='ls-item '+fileTypeClass(n);s.innerHTML=fileIcon(n)+n;d.appendChild(s)
  })
  return d
}

function fileTypeClass(name){
  if(name.endsWith('/'))return 'dir'
  var ext=name.split('.').pop().toLowerCase()
  if(['go','py','js','ts','jsx','tsx','rs','rb','java','c','cpp','h','hpp','cs','swift','kt','php','pl','r','dart','lua','hs','clj','scala','ex','exs','html','htm','css','scss','sass','less','vue','svelte','xml'].includes(ext))return'go'
  if(['sh','bash','zsh','fish'].includes(ext))return'sh'
  if(['md','txt','markdown','pdf','doc','docx','xls','xlsx','ppt','pptx','csv','epub','log'].includes(ext))return'md'
  if(['png','jpg','jpeg','gif','svg','webp','ico','bmp','tiff','tif','raw'].includes(ext))return'img'
  if(['zip','tar','gz','bz2','xz','7z','rar','iso','deb','rpm'].includes(ext))return'zip'
  if(['db','sqlite','sqlite3'].includes(ext))return'db'
  if(['json','yaml','yml','toml','ini','cfg','conf','ttf','otf','woff','woff2','eot'].includes(ext))return'cfg'
  if(['mp3','wav','flac','aac','ogg','m4a','wma'].includes(ext))return'audio'
  if(['mp4','avi','mkv','mov','wmv','webm','flv'].includes(ext))return'video'
  if(ext===''||['exe','bin','AppImage'].includes(ext))return'exe'
  return''
}

// init prompt — render before async fetch
renderPrompt($('inpPrompt'))
fetchPromptInfo()



/* --- shortcuts --- */
document.querySelectorAll('#tab-shortcuts .mod-btn').forEach(btn=>{
  btn.onclick=function(){
    const m=this.dataset.mod;const i=scMods.indexOf(m)
    i>=0?scMods.splice(i,1):scMods.push(m)
    this.classList.toggle('active')
    updateScPreview()
  }
})
$('scKey').oninput=function(){
  this.value=this.value===' '||this.value==='\t'?' ':this.value.slice(-1).toLowerCase()
  document.querySelectorAll('.key-btn').forEach(b=>b.classList.toggle('active',b.dataset.key===this.value))
  updateScPreview()
}
$('scKey').onkeydown=e=>{if(e.key==='Enter')$('scAddBtn').click()}
document.querySelectorAll('#tab-shortcuts .key-btn').forEach(btn=>{
  btn.onclick=function(){$('scKey').value=this.dataset.key;document.querySelectorAll('.key-btn').forEach(b=>b.classList.toggle('active',b.dataset.key===this.dataset.key));updateScPreview()}
})
function scKeyName(){return $('scKey').value?($('scKey').value===' '?'space':$('scKey').value):''}
function updateScPreview(){const p=[...scMods],k=scKeyName();if(k)p.push(k);$('scPreview').textContent=p.join('+')}
function displayCombo(c){if(!c)return'';const m={'ctrl':'Ctrl','alt':'Alt','shift':'Shift','meta':'Cmd'};return c.replace(/(ctrl|alt|shift|meta)/g,(_,k)=>m[k]||k).replace('space','Space')}
function loadShortcuts(){
  api('GET','/shortcuts').then(list=>{
    if(!list||!list.length){$('scList').innerHTML='<div class="empty-state">No shortcuts</div>';return}
    $('scList').innerHTML=list.map(s=>'<div class="file-item" style="cursor:default"><span class="name" style="flex:1">'+escapeHtml(s.name)+'</span><span style="font-family:monospace;font-size:11px;color:var(--accent)">'+displayCombo(s.combo)+'</span><button class="sc-run" data-combo="'+escapeAttr(s.combo)+'" style="margin-left:6px;font-size:10px;padding:2px 6px">Run</button><button class="sc-del" data-id="'+s.id+'" style="color:var(--red);background:none;border:none;cursor:pointer;font-size:14px;padding:0 4px;margin-left:2px">x</button></div>').join('')
  }).catch(()=>{})
}
$('scAddBtn').onclick=function(){
  const name=$('scName').value.trim()
  const parts=[...scMods];const k=scKeyName();if(k)parts.push(k);const combo=parts.join('+')
  if(!name||!combo)return
  api('POST','/shortcuts',{name,combo}).then(()=>{$('scName').value='';$('scKey').value='';scMods=[];document.querySelectorAll('.mod-btn,.key-btn').forEach(b=>b.classList.remove('active'));$('scPreview').textContent='';loadShortcuts()}).catch(e=>showToast(e.message,'error'))
}
$('scName').onkeydown=e=>{if(e.key==='Enter')$('scAddBtn').click()}
$('scList').onclick=function(e){
  const btn=e.target.closest('button')
  if(!btn)return
  if(btn.classList.contains('sc-del')){fetch(BASE+'/shortcuts/'+btn.dataset.id,{method:'DELETE'}).then(()=>loadShortcuts()).catch(()=>{})}
  else if(btn.classList.contains('sc-run')){api('POST','/key',{combo:btn.dataset.combo}).catch(e=>showToast(e.message,'error'))}
}

/* --- toast & modal --- */
function showToast(msg,type){
  const t=$('toast')
  t.textContent=msg
  t.className='toast '+(type||'')
  void t.offsetWidth
  t.classList.add('show')
  clearTimeout(t._timer)
  t._timer=setTimeout(()=>t.classList.remove('show'),3000)
}
function showModal(opts){
  return new Promise(resolve=>{
    const overlay=$('modalOverlay')
    $('modalTitle').textContent=opts.title||''
    if(opts.message){
      $('modalMsg').style.display=''
      $('modalMsg').textContent=opts.message
      $('modalInput').style.display='none'
    }else{
      $('modalMsg').style.display='none'
      $('modalInput').style.display=''
      $('modalInput').value=opts.value||''
      $('modalInput').placeholder=opts.placeholder||''
    }
    $('modalOk').textContent=opts.okText||'OK'
    $('modalCancel').textContent=opts.cancelText||'Cancel'
    overlay.style.display='flex'
    if(opts.message){$('modalCancel').focus()}else{$('modalInput').focus();$('modalInput').select()}
    function done(val){overlay.style.display='none';resolve(val)}
    $('modalOk').onclick=()=>{const val=opts.message?true:$('modalInput').value;done(val)}
    $('modalCancel').onclick=()=>done(opts.message?false:null)
    overlay.onclick=e=>{if(e.target===overlay)done(opts.message?false:null)}
    $('modalInput').onkeydown=e=>{
      if(e.key==='Enter')$('modalOk').click()
      if(e.key==='Escape')$('modalCancel').click()
    }
  })
}

/* --- utils --- */
function escapeHtml(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function escapeAttr(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
function escapeRegex(s){return s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}

/* --- apps --- */
function loadApps(filter){
  const grid=$('appsGrid')
  grid.innerHTML='<div class="empty-state">Loading...</div>'
  api('POST','/apps',{filter:filter||''}).then(r=>{
    if(!r.apps||!r.apps.length){grid.innerHTML='<div class="empty-state">No apps found</div>';return}
    r.apps.sort((a,b)=>a.name.localeCompare(b.name))
    var html=''
    r.apps.forEach((app,i)=>{
      const letter=(app.name||'?')[0].toUpperCase()
      const delay=Math.min(i*0.02,0.5)
      html+='<div class="app-item" style="animation-delay:'+delay+'s" data-name="'+escapeAttr(app.name)+'">'
        +'<div class="app-icon">'+escapeHtml(letter)+'</div>'
        +'<div class="app-name">'+escapeHtml(app.name)+'</div>'
        +(app.comment?'<div class="app-comment">'+escapeHtml(app.comment)+'</div>':'')
        +'</div>'
    })
    grid.innerHTML=html
    grid.querySelectorAll('.app-item').forEach(el=>{
      el.onclick=function(){
        const name=this.dataset.name
        api('POST','/apps/launch',{name:name})
          .then(()=>{showToast('Launched: '+name,'success');setTimeout(loadActiveApps,1000)})
          .catch(e=>showToast('Launch error: '+e.message,'error'))
      }
    })
  }).catch(e=>{grid.innerHTML='<div class="empty-state">Error: '+escapeHtml(e.message)+'</div>'})
}
$('appsSearch').oninput=function(){clearTimeout(this._debounce);this._debounce=setTimeout(()=>loadApps(this.value.trim()),300)}

/* --- active apps (via windows) --- */
var _lastActiveAppsJSON=''
var _activeAppsAnim=true
function loadActiveApps(anim){
  if(anim===undefined) anim=_activeAppsAnim
  _activeAppsAnim=false
  const grid=$('activeAppsGrid')
  if(!grid) return
  api('POST','/windows',{}).then(r=>{
    const wins=Array.isArray(r)?r:(r.windows||[])
    const json=JSON.stringify(wins)
    if(json===_lastActiveAppsJSON) return
    _lastActiveAppsJSON=json
    if(!wins.length){grid.innerHTML='<div class="empty-state">No windows open</div>';return}
    var html=''
    wins.forEach(w=>{
      const name=w.app||w.title||'Unknown'
      const letter=(name)[0].toUpperCase()
      html+='<div class="active-app-item'+(w.active?' active':'')+(anim?'':' no-anim')+'" data-pid="'+(w.pid||0)+'">'
        +'<div class="active-app-icon">'+escapeHtml(letter)+'</div>'
        +'<div class="active-app-info">'
        +'<div class="active-app-name">'+escapeHtml(name)+'</div>'
        +'<div class="active-app-detail">'+escapeHtml(w.title||'')+'</div>'
        +'</div>'
        +(w.pid?'<button class="active-app-close" onclick="closeApp('+w.pid+')">&times;</button>':'')
        +'</div>'
    })
    grid.innerHTML=html
  }).catch(()=>{})
}
function closeApp(pid){
  api('POST','/exec',{cmd:['sh','-c','kill '+pid],timeout_sec:5}).then(()=>{loadActiveApps()}).catch(e=>showToast('Close error: '+e.message,'error'))
}

/* --- init --- */
checkAuth()
setInterval(function(){if(authenticated)fetch(BASE+'/devices').then(r=>r.json()).then(onDevices).catch(()=>{})},3000)
setInterval(function(){if(authenticated&&deviceId)loadActiveApps()},5000)
