(function(a,b){'function'==typeof define&&define.amd?define([],b):'undefined'!=typeof module&&module.exports?module.exports=b():a.ReconnectingWebSocket=b()})(this,function(){function a(b,c,d){function f(n,o){var p=document.createEvent('CustomEvent');return p.initCustomEvent(n,!1,!1,o),p}var g={debug:!1,automaticOpen:!0,reconnectInterval:1e3,maxReconnectInterval:3e4,reconnectDecay:1.5,timeoutInterval:2e3,maxReconnectAttempts:null,binaryType:'blob'};for(var h in d||(d={}),g)this[h]='undefined'==typeof d[h]?g[h]:d[h];this.url=b,this.reconnectAttempts=0,this.readyState=WebSocket.CONNECTING,this.protocol=null;var j,i=this,k=!1,l=!1,m=document.createElement('div');m.addEventListener('open',function(n){i.onopen(n)}),m.addEventListener('close',function(n){i.onclose(n)}),m.addEventListener('connecting',function(n){i.onconnecting(n)}),m.addEventListener('message',function(n){i.onmessage(n)}),m.addEventListener('error',function(n){i.onerror(n)}),this.addEventListener=m.addEventListener.bind(m),this.removeEventListener=m.removeEventListener.bind(m),this.dispatchEvent=m.dispatchEvent.bind(m),this.open=function(n){if(j=new WebSocket(i.url,c||[]),j.binaryType=this.binaryType,!n)m.dispatchEvent(f('connecting')),this.reconnectAttempts=0;else if(this.maxReconnectAttempts&&this.reconnectAttempts>this.maxReconnectAttempts)return;(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','attempt-connect',i.url);var o=j,p=setTimeout(function(){(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','connection-timeout',i.url),l=!0,o.close(),l=!1},i.timeoutInterval);j.onopen=function(){clearTimeout(p),(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','onopen',i.url),i.protocol=j.protocol,i.readyState=WebSocket.OPEN,i.reconnectAttempts=0;var r=f('open');r.isReconnect=n,n=!1,m.dispatchEvent(r)},j.onclose=function(q){if(clearTimeout(t),j=null,k)i.readyState=WebSocket.CLOSED,m.dispatchEvent(f('close'));else{i.readyState=WebSocket.CONNECTING;var r=f('connecting');r.code=q.code,r.reason=q.reason,r.wasClean=q.wasClean,m.dispatchEvent(r),n||l||((i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','onclose',i.url),m.dispatchEvent(f('close')));var t=i.reconnectInterval*Math.pow(i.reconnectDecay,i.reconnectAttempts);setTimeout(function(){i.reconnectAttempts++,i.open(!0)},t>i.maxReconnectInterval?i.maxReconnectInterval:t)}},j.onmessage=function(q){(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','onmessage',i.url,q.data);var r=f('message');r.data=q.data,m.dispatchEvent(r)},j.onerror=function(q){(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','onerror',i.url,q),m.dispatchEvent(f('error'))}},!0==this.automaticOpen&&this.open(!1),this.send=function(n){if(j)return(i.debug||a.debugAll)&&console.debug('ReconnectingWebSocket','send',i.url,n),j.send(n);throw'INVALID_STATE_ERR : Pausing to reconnect websocket'},this.close=function(n,o){'undefined'==typeof n&&(n=1e3),k=!0,j&&j.close(n,o)},this.refresh=function(){j&&j.close()}}if('WebSocket'in window)return a.prototype.onopen=function(){},a.prototype.onclose=function(){},a.prototype.onconnecting=function(){},a.prototype.onmessage=function(){},a.prototype.onerror=function(){},a.debugAll=!1,a.CONNECTING=WebSocket.CONNECTING,a.OPEN=WebSocket.OPEN,a.CLOSING=WebSocket.CLOSING,a.CLOSED=WebSocket.CLOSED,a});
