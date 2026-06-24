/* --- files (iOS style) --- */
var fileHistory=[],fileHistIdx=-1

$('fileBack').onclick=function(){
  if(fileHistIdx>0){fileHistIdx--;listDir(fileHistory[fileHistIdx],true)}
}
$('fileFwd').onclick=function(){
  if(fileHistIdx<fileHistory.length-1){fileHistIdx++;listDir(fileHistory[fileHistIdx],true)}
}
$('fileUploadBtn').onclick=function(){$('fileUploadInput').click()}
$('fileUploadInput').onchange=function(){
  const file=this.files[0];if(!file)return
  const reader=new FileReader()
  reader.onload=function(e){
    const cur=fileHistory[fileHistIdx]||'/'
    const path=(cur.endsWith('/')?cur:cur+'/')+file.name
    const raw=e.target.result
    api('POST','/write',{path,content:raw.substring(raw.indexOf(',')+1),base64:true}).then(()=>{listDir(cur);showToast('Uploaded: '+file.name,'success')}).catch(err=>showToast('Upload error: '+err.message,'error'))
  }
  reader.readAsDataURL(file)
  this.value=''
}

function listDir(path,silent){
  if(!path)return
  if(!silent){
    fileHistory=fileHistory.slice(0,fileHistIdx+1)
    fileHistory.push(path)
    fileHistIdx=fileHistory.length-1
  }
  $('fileBack').disabled=fileHistIdx<=0
  $('fileFwd').disabled=fileHistIdx>=fileHistory.length-1
  renderBreadcrumb(path)
  api('POST','/ls',{path}).then(r=>{
    if(!r.entries||!r.entries.length){
      $('fileList').innerHTML='<div class="empty-state">Empty directory</div>'
      return
    }
    r.entries.sort(function(a,b){
      var aDir=a.is_dir||(a.mode_string&&a.mode_string.startsWith('d'))
      var bDir=b.is_dir||(b.mode_string&&b.mode_string.startsWith('d'))
      if(aDir!==bDir) return aDir?-1:1
      return a.name.localeCompare(b.name,void 0,{numeric:true,sensitivity:'base'})
    })
    var html=''
    r.entries.forEach((e,i)=>{
      const isDir=e.is_dir||(e.mode_string&&e.mode_string.startsWith('d'))
      const fn=e.name+(isDir?'/':'')
      const delay=Math.min(i*0.03,0.4)
      html+='<div class="file-grid-item '+fileTypeClass(fn)+'" style="animation-delay:'+delay+'s" data-name="'+escapeAttr(e.name)+'" data-dir="'+isDir+'">'
      +'<div class="icon">'+fileIcon(fn)+'</div>'
      +'<div class="name">'+escapeHtml(e.name)+'</div>'
      +'</div>'
    })
    $('fileList').innerHTML=html
    $('fileList').oncontextmenu=function(e){e.preventDefault()}
    $('fileList').querySelectorAll('.file-grid-item').forEach(el=>{
      el.onclick=function(){
        const name=this.dataset.name
        if(this.dataset.dir==='true'){
          listDir((path.endsWith('/')?path:path+'/')+name)
        }else{
          openFileSheet((path.endsWith('/')?path:path+'/')+name)
        }
      }
    })
    $('fileList').oncontextmenu=function(e){
      e.preventDefault()
      const item=e.target.closest('.file-grid-item')
      if(item){
        item.classList.add('selected')
        document.querySelectorAll('#fileContextMenu .ctx-item').forEach(function(el){
          el.style.display=el.dataset.action==='new-folder'?'none':''
        })
        $('fileContextMenu').querySelector('.ctx-sep').style.display=''
        ctxIsDir=item.dataset.dir==='true'
        ctxName=item.dataset.name
        ctxPath=(path.endsWith('/')?path:path+'/')+ctxName
      }else{
        document.querySelectorAll('#fileContextMenu .ctx-item').forEach(function(el){
          el.style.display=el.dataset.action==='new-folder'?'':'none'
        })
        $('fileContextMenu').querySelector('.ctx-sep').style.display='none'
        ctxIsDir=true
        ctxName='[new folder]'
        ctxPath=fileHistory[fileHistIdx]||'/'
      }
      const menu=$('fileContextMenu')
      menu.style.left=e.clientX+'px'
      menu.style.top=e.clientY+'px'
      menu.style.display='block'
    }
  }).catch(e=>{console.log('listDir error:',e.message);$('fileList').innerHTML='<div class="empty-state">Error: '+escapeHtml(e.message)+'</div>'})
}

function renderBreadcrumb(path){
  const parts=path.split('/').filter(Boolean)
  var html='<span class="file-path-seg" data-path="/">/</span>'
  var cur=''
  parts.forEach(p=>{
    cur+='/'+p
    html+='<span class="file-path-sep">▸</span><span class="file-path-seg" data-path="'+escapeAttr(cur)+'">'+escapeHtml(p)+'</span>'
  })
  $('fileBreadcrumb').innerHTML=html
  $('fileBreadcrumb').querySelectorAll('.file-path-seg').forEach(el=>{
    el.onclick=function(){listDir(this.dataset.path)}
  })
}

function openFileSheet(fp){
  const ext=fp.split('.').pop().toLowerCase()
  const isImg=['png','jpg','jpeg','gif','svg','webp','ico'].includes(ext)
  const isPdf=ext==='pdf'
  const isVideo=['mp4','webm','ogg','ogv'].includes(ext)
  const isAudio=['mp3','wav','flac','m4a','aac'].includes(ext)
  const isPreview=isImg||isPdf||isVideo||isAudio
  $('fileSheetTitle').textContent=fp.split('/').pop()
  $('fileContent').dataset.path=fp
  $('fileSheetOverlay').style.display='block'
  $('fileSheet').style.display='flex'
  if(isPreview){
    $('fileContent').style.display='none'
    $('fileImagePreview').style.display='flex'
    $('fileImagePreview').innerHTML='<div style="color:var(--dim)">Loading…</div>'
    $('fileSheetSave').style.display='none'
  }else{
    $('fileContent').value='loading...'
    $('fileContent').style.display=''
    $('fileImagePreview').style.display='none'
    $('fileSheetSave').style.display=''
  }
  requestAnimationFrame(()=>{
    $('fileSheetOverlay').classList.add('show')
    $('fileSheet').classList.add('show')
  })
  api('POST','/read',{path:fp}).then(r=>{
    if(isPreview&&r.base64){
      if(isImg){
        const mime=ext==='svg'?'image/svg+xml':ext==='webp'?'image/webp':ext==='ico'?'image/x-icon':'image/'+(ext==='jpg'?'jpeg':ext)
        $('fileImagePreview').innerHTML='<img src="data:'+mime+';base64,'+r.content+'" alt="preview" />'
      }else if(isPdf){
        $('fileImagePreview').innerHTML='<embed src="data:application/pdf;base64,'+r.content+'" type="application/pdf" style="width:100%;height:100%;border:none" />'
      }else if(isVideo){
        const mime=ext==='mp4'?'video/mp4':ext==='webm'?'video/webm':'video/ogg'
        $('fileImagePreview').innerHTML='<video controls style="max-width:100%;max-height:100%" src="data:'+mime+';base64,'+r.content+'"></video>'
      }else if(isAudio){
        const mime=ext==='mp3'?'audio/mpeg':ext==='wav'?'audio/wav':ext==='flac'?'audio/flac':ext==='m4a'?'audio/mp4':'audio/aac'
        $('fileImagePreview').innerHTML='<audio controls style="width:100%" src="data:'+mime+';base64,'+r.content+'"></audio>'
      }
    }else{
      $('fileContent').value=r.content||''
    }
  }).catch(e=>{$('fileContent').value='Error: '+e.message})
}
function closeFileSheet(){
  $('fileSheet').classList.remove('show')
  $('fileSheetOverlay').classList.remove('show')
  setTimeout(()=>{$('fileSheet').style.display='none';$('fileSheetOverlay').style.display='none'},300)
}
$('fileSheetClose').onclick=closeFileSheet
$('fileSheetOverlay').onclick=closeFileSheet
$('fileSheetSave').onclick=function(){
  const fp=$('fileContent').dataset.path
  const content=$('fileContent').value
  if(!fp)return
  api('POST','/write',{path:fp,content}).then(()=>{closeFileSheet();listDir(fileHistory[fileHistIdx]);showToast('Saved','success')}).catch(e=>showToast('Save error: '+e.message,'error'))
}

/* --- context menu --- */
var ctxPath='',ctxName='',ctxIsDir=false

function hideContextMenu(){
  $('fileContextMenu').style.display='none'
  $('fileList').querySelectorAll('.selected').forEach(function(el){el.classList.remove('selected')})
  document.querySelectorAll('#fileContextMenu .ctx-item').forEach(function(el){el.style.display=''})
  $('fileContextMenu').querySelector('.ctx-sep').style.display=''
}
document.addEventListener('click',function(e){
  if(!e.target.closest('.file-context-menu')) hideContextMenu()
})
document.addEventListener('keydown',function(e){
  if(e.key==='Escape') hideContextMenu()
})

$('fileContextMenu').onclick=async function(e){
  const item=e.target.closest('.ctx-item')
  if(!item)return
  const action=item.dataset.action
  hideContextMenu()
  switch(action){
    case 'open':{
      if(ctxIsDir) listDir(ctxPath)
      else openFileSheet(ctxPath)
      break
    }
    case 'download':{
      $('fileContent').value='downloading...'
      openFileSheet(ctxPath)
      api('POST','/read',{path:ctxPath}).then(r=>{
        const blob=new Blob([r.content||''],{type:'application/octet-stream'})
        const a=document.createElement('a')
        a.href=URL.createObjectURL(blob)
        a.download=ctxName
        a.click()
        URL.revokeObjectURL(a.href)
        closeFileSheet()
      }).catch(e=>{showToast('Download error: '+e.message,'error');closeFileSheet()})
      break
    }
    case 'copy-path':{
      navigator.clipboard.writeText(ctxPath).catch(()=>{})
      break
    }
    case 'rename':{
      const newName=await showModal({title:'Rename',input:true,value:ctxName,okText:'Rename'})
      if(!newName||newName===ctxName) return
      const parent=ctxPath.slice(0,ctxPath.lastIndexOf('/'))
      const newPath=parent+'/'+newName
      api('POST','/exec',{cmd:['sh','-c','mv '+quote(ctxPath)+' '+quote(newPath)],timeout_sec:10})
      .then(()=>listDir(fileHistory[fileHistIdx]))
      .catch(e=>showToast('Rename error: '+e.message,'error'))
      break
    }
    case 'delete':{
      const ok=await showModal({title:'Delete',message:'Delete "'+ctxName+'"?',okText:'Delete'})
      if(!ok) return
      const flag=ctxIsDir?'-rf ':''
      api('POST','/exec',{cmd:['sh','-c','rm '+flag+quote(ctxPath)],timeout_sec:10})
      .then(()=>listDir(fileHistory[fileHistIdx]))
      .catch(e=>showToast('Delete error: '+e.message,'error'))
      break
    }
    case 'new-folder':{
      const folderName=await showModal({title:'New Folder',input:true,okText:'Create'})
      if(!folderName) return
      const parent=fileHistory[fileHistIdx]||'/'
      const newPath=(parent.endsWith('/')?parent:parent+'/')+folderName
      api('POST','/exec',{cmd:['sh','-c','mkdir -p '+quote(newPath)],timeout_sec:10})
      .then(()=>listDir(parent))
      .catch(e=>showToast('Error: '+e.message,'error'))
      break
    }
  }
}
