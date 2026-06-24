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
    if(tab) tab.classList.add('active')
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
}
$('qaExec').onclick=function(){
  api('POST','/exec',{cmd:['sh','-c','uname -a'],timeout_sec:5}).then(r=>{
    const p=$('dashShotWrap').querySelector('.placeholder')
    if(r.stdout) p.textContent=r.stdout;else p.textContent='(no output)'
    p.style.display='block';$('dashShotCanvas').style.display='none'
  }).catch(e=>{$('dashShotWrap').querySelector('.placeholder').textContent='Error: '+e.message})
  $('dashScreenshotSection').style.display='block'
  $('dashShotWrap').querySelector('.placeholder').style.display='block'
  $('dashShotCanvas').style.display='none'
}
$('qaScreenshot').onclick=function(){
  api('POST','/screenshot',{}).then(r=>{
    $('dashScreenshotSection').style.display='block'
    const p=$('dashShotWrap').querySelector('.placeholder')
    const c=$('dashShotCanvas')
    if(r.base64){
      p.style.display='none';c.style.display='block'
      const img=new Image()
      img.onload=function(){c.width=img.width;c.height=img.height;c.getContext('2d').drawImage(img,0,0);img.remove()}
      img.src='data:image/png;base64,'+r.base64
    }else{p.style.display='block';c.style.display='none';p.textContent='(no image data)'}
  }).catch(e=>{const p=$('dashShotWrap').querySelector('.placeholder');p.style.display='block';$('dashShotCanvas').style.display='none';p.textContent='Error: '+e.message})
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
  api('POST','/mouse',{x,y}).then(()=>{$('clickPos').textContent=x+','+y+' moved'}).catch(e=>alert('Mouse: '+e.message))
}
$('clickLeft').onclick=()=>api('POST','/click',{button:'left'}).catch(e=>alert(e.message))
$('clickRight').onclick=()=>api('POST','/click',{button:'right'}).catch(e=>alert(e.message))
$('clickMiddle').onclick=()=>api('POST','/click',{button:'middle'}).catch(e=>alert(e.message))
$('clickDouble').onclick=()=>api('POST','/click',{button:'left',double:true}).catch(e=>alert(e.message))
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
$('typeBtn').onclick=function(){const t=$('typeText').value;if(t)api('POST','/type',{text:t}).catch(e=>alert(e.message))}
$('typeText').onkeydown=e=>{if(e.key==='Enter')$('typeBtn').click()}
document.querySelectorAll('#tab-remote button[data-combo]').forEach(btn=>{btn.onclick=function(){api('POST','/key',{combo:this.dataset.combo}).catch(e=>alert(e.message))}})

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

$('shellCmd').onkeydown=function(e){
  if(e.key==='Enter'){
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

function renderCmdLine(cmd){
  const promptLine=document.createElement('div');promptLine.className='term-line'
  promptLine.innerHTML='<div class="prompt">'+renderPromptStr()+'</div>'
  const cmdLine=document.createElement('div');cmdLine.className='term-line'
  cmdLine.innerHTML='<span style="color:var(--green);font-weight:600">-&gt;</span> <span class="cmd">'+escapeHtml(cmd)+'</span>'
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

function lsGrid(text){
  const items=text.split(/\s+/).filter(Boolean)
  const d=document.createElement('div');d.className='ls-grid'
  items.forEach(n=>{
    const s=document.createElement('span');s.className='ls-item '+fileTypeClass(n);s.textContent=n;d.appendChild(s)
  })
  return d
}

function fileTypeClass(name){
  if(name.endsWith('/'))return 'dir'
  var ext=name.split('.').pop().toLowerCase()
  if(ext==='go')return'go'
  if(['sh','bash','zsh','fish'].includes(ext))return'sh'
  if(['md','txt','markdown'].includes(ext))return'md'
  if(['png','jpg','jpeg','gif','svg','webp','ico'].includes(ext))return'img'
  if(['zip','tar','gz','bz2','xz','7z','rar'].includes(ext))return'zip'
  if(['db','sqlite','sqlite3'].includes(ext))return'db'
  if(['json','yaml','yml','toml','ini','cfg','conf'].includes(ext))return'cfg'
  if(ext===''||['exe','bin','AppImage'].includes(ext))return'exe'
  return''
}

// init prompt — render before async fetch
renderPrompt($('inpPrompt'))
fetchPromptInfo()

/* --- files --- */
$('fileLsBtn').onclick=function(){listDir($('filePath').value)}
$('filePath').onkeydown=e=>{if(e.key==='Enter')$('fileLsBtn').click()}
function listDir(path){
  if(!path)return
  $('filePath').value=path
  api('POST','/ls',{path:path}).then(r=>{
    var html=''
    if(!r.entries||!r.entries.length){html='<div class="empty-state">Empty directory</div>'}
    else{
      r.entries.forEach(e=>{
        const isDir=e.is_dir||e.mode_string&&e.mode_string.startsWith('d')
        const icon=isDir?'<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z"/></svg>':'<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>'
        const size=e.size!==undefined?formatSize(e.size):''
        html+='<div class="file-item" data-name="'+escapeAttr(e.name)+'" data-dir="'+isDir+'"><span class="icon">'+icon+'</span><span class="name">'+escapeHtml(e.name)+'</span><span class="size">'+size+'</span></div>'
      })
    }
    $('fileList').innerHTML=html
    $('fileList').querySelectorAll('.file-item').forEach(el=>{
      var p=$('filePath').value
      if(!p.endsWith('/'))p+='/'
      el.onclick=function(){
        const name=this.dataset.name
        if(this.dataset.dir==='true'){listDir(p+name)}
        else{const fp=p+name;$('fileReadPath').value=fp;readFile(fp)}
      }
    })
  }).catch(e=>{$('fileList').innerHTML='<div class="empty-state">Error: '+escapeHtml(e.message)+'</div>'})
}
function readFile(path){
  $('fileReadPath').value=path
  api('POST','/read',{path:path}).then(r=>{$('fileContent').value=r.content||''}).catch(e=>{$('fileContent').value='Error: '+e.message})
}
$('fileReadBtn').onclick=function(){readFile($('fileReadPath').value)}
$('fileReadPath').onkeydown=e=>{if(e.key==='Enter')$('fileReadBtn').click()}
$('fileSaveBtn').onclick=function(){
  const path=$('fileReadPath').value;const content=$('fileContent').value
  if(!path)return
  api('POST','/write',{path:path,content:content}).then(()=>{$('fileReadPath').value='';$('fileContent').value='';listDir($('filePath').value)}).catch(e=>alert('Save error: '+e.message))
}
$('fileUploadBtn').onclick=function(){$('fileUploadInput').click()}
$('fileUploadInput').onchange=function(){
  const file=this.files[0];if(!file)return
  const reader=new FileReader()
  reader.onload=function(e){
    const path=($('filePath').value.endsWith('/')?$('filePath').value:$('filePath').value+'/')+file.name
    api('POST','/write',{path:path,content:e.target.result}).then(()=>{listDir($('filePath').value)}).catch(err=>alert('Upload error: '+err.message))
  }
  reader.readAsText(file)
  this.value=''
}

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
  api('POST','/shortcuts',{name,combo}).then(()=>{$('scName').value='';$('scKey').value='';scMods=[];document.querySelectorAll('.mod-btn,.key-btn').forEach(b=>b.classList.remove('active'));$('scPreview').textContent='';loadShortcuts()}).catch(e=>alert(e.message))
}
$('scName').onkeydown=e=>{if(e.key==='Enter')$('scAddBtn').click()}
$('scList').onclick=function(e){
  const btn=e.target.closest('button')
  if(!btn)return
  if(btn.classList.contains('sc-del')){fetch(BASE+'/shortcuts/'+btn.dataset.id,{method:'DELETE'}).then(()=>loadShortcuts()).catch(()=>{})}
  else if(btn.classList.contains('sc-run')){api('POST','/key',{combo:btn.dataset.combo}).catch(e=>alert(e.message))}
}

/* --- utils --- */
function escapeHtml(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function escapeAttr(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
function escapeRegex(s){return s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}
function formatSize(n){if(!n&&n!==0)return'';if(n<1024)return n+'B';if(n<1024*1024)return(n/1024).toFixed(1)+'K';return(n/1024/1024).toFixed(1)+'M'}

/* --- init --- */
checkAuth()
setInterval(function(){if(authenticated)fetch(BASE+'/devices').then(r=>r.json()).then(onDevices).catch(()=>{})},3000)
