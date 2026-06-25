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
