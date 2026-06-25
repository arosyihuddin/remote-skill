/* --- utils --- */
function escapeHtml(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function escapeAttr(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;')}
function escapeRegex(s){return s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}

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

/* --- file icons --- */
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
  if(['json','yaml','yml','toml','ini','cfg','conf','ttf','otf','woff','woff2','eot'].includes(ext)) return '<svg class="icon" width="14" height="14" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M11.0779 2.25C10.1613 2.25 9.37909 2.91265 9.22841 3.81675L9.04974 4.88873C9.02959 5.00964 8.93542 5.1498 8.75311 5.23747C8.40905 5.40292 8.07967 5.5938 7.7674 5.8076C7.60091 5.92159 7.43259 5.9332 7.31769 5.89015L6.29851 5.50833C5.44019 5.18678 4.4752 5.53289 4.01692 6.32666L3.09493 7.92358C2.63665 8.71736 2.8194 9.72611 3.52704 10.3087L4.36756 11.0006C4.46219 11.0785 4.53629 11.2298 4.52119 11.4307C4.50706 11.6188 4.49988 11.8086 4.49988 12C4.49988 12.1915 4.50707 12.3814 4.52121 12.5695C4.53632 12.7704 4.46221 12.9217 4.36758 12.9996L3.52704 13.6916C2.8194 14.2741 2.63665 15.2829 3.09493 16.0767L4.01692 17.6736C4.4752 18.4674 5.44019 18.8135 6.29851 18.4919L7.31791 18.11C7.43281 18.067 7.60113 18.0786 7.76761 18.1925C8.07982 18.4063 8.40913 18.5971 8.75311 18.7625C8.93542 18.8502 9.02959 18.9904 9.04974 19.1113L9.22841 20.1832C9.37909 21.0874 10.1613 21.75 11.0779 21.75H12.9219C13.8384 21.75 14.6207 21.0874 14.7713 20.1832L14.95 19.1113C14.9702 18.9904 15.0643 18.8502 15.2466 18.7625C15.5907 18.5971 15.9201 18.4062 16.2324 18.1924C16.3988 18.0784 16.5672 18.0668 16.6821 18.1098L17.7012 18.4917C18.5596 18.8132 19.5246 18.4671 19.9828 17.6733L20.9048 16.0764C21.3631 15.2826 21.1804 14.2739 20.4727 13.6913L19.6322 12.9994C19.5376 12.9215 19.4635 12.7702 19.4786 12.5693C19.4927 12.3812 19.4999 12.1914 19.4999 12C19.4999 11.8085 19.4927 11.6186 19.4785 11.4305C19.4634 11.2296 19.5375 11.0783 19.6322 11.0004L20.4727 10.3084C21.1804 9.72587 21.3631 8.71711 20.9048 7.92334L19.9828 6.32642C19.5246 5.53264 18.5596 5.18654 17.7012 5.50809L16.6818 5.89C16.5669 5.93304 16.3986 5.92144 16.2321 5.80746C15.9199 5.59371 15.5906 5.40289 15.2466 5.23747C15.0643 5.1498 14.9702 5.00964 14.95 4.88873L14.7713 3.81675C14.6207 2.91265 13.8384 2.25 12.9219 2.25H11.0779ZM12 15C14.7614 15 17 12.7614 17 10C17 7.23858 14.7614 5 12 5C9.23858 5 7 7.23858 7 10C7 12.7614 9.23858 15 12 15Z"/></svg>'
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
        +(w.window_id?'<button class="active-app-close" onclick="closeApp(\''+w.window_id+'\','+w.pid+')">&times;</button>':w.pid?'<button class="active-app-close" onclick="closeApp(null,'+w.pid+')">&times;</button>':'')
        +'</div>'
    })
    grid.innerHTML=html
  }).catch(()=>{})
}
function closeApp(windowId,pid){
  if(windowId){
    api('POST','/close-window',{window_id:windowId}).then(()=>{loadActiveApps()}).catch(e=>{
      if(pid){api('POST','/exec',{cmd:['sh','-c','kill '+pid],timeout_sec:5}).then(()=>{loadActiveApps()}).catch(e2=>showToast('Close error: '+e2.message,'error'))}
      else{showToast('Close error: '+e.message,'error')}
    })
  }else if(pid){
    api('POST','/exec',{cmd:['sh','-c','kill '+pid],timeout_sec:5}).then(()=>{loadActiveApps()}).catch(e=>showToast('Close error: '+e.message,'error'))
  }
}

/* --- init --- */
checkAuth()
setInterval(function(){if(authenticated)fetch(BASE+'/devices').then(r=>r.json()).then(onDevices).catch(()=>{})},3000)
setInterval(function(){if(authenticated&&deviceId)loadActiveApps()},5000)
