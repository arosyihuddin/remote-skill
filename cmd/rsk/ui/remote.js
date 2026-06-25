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
