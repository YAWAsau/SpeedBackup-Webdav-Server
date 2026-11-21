/* Shared module starfield: local canvas only; no network or backend work. */
(function () {
'use strict';
    var canvas = document.createElement('canvas');
    canvas.id = 'theme-sky';
    canvas.setAttribute('aria-hidden', 'true');
    document.body.prepend(canvas);
    var ctx = canvas.getContext('2d');
    var contrast = matchMedia('(forced-colors: active)');
    var frame = 0, last = 0, phase = 0, width = 0, height = 0;
    var suspended = false;
    var palette = [190, 265, 320, 35, 145, 215];
    var glows = [];
    // Normalized positions keep resizing stable and cap the amount of drawing.
    var stars = Array.from({ length: 200 }, function () {
      return { x: Math.random(), y: Math.random(), r: 0.7 + Math.random() * 1.4, speed: 0.4 + Math.random(), hue: palette[Math.floor(Math.random() * palette.length)] };
    });
    function size() {
      width = window.innerWidth; height = window.innerHeight;
      var dpr = Math.min(window.devicePixelRatio || 1, 1.5);
      canvas.width = Math.round(width * dpr); canvas.height = Math.round(height * dpr);
      if (ctx) {
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        glows = palette.slice(0, 4).map(function (hue) {
          var radius = Math.min(width, height) * 0.8;
          var glow = ctx.createRadialGradient(0, 0, 0, 0, 0, radius);
          glow.addColorStop(0, 'hsla(' + hue + ',95%,58%,0.22)');
          glow.addColorStop(1, 'hsla(' + hue + ',95%,45%,0)');
          return { paint: glow, radius: radius };
        });
      }
    }
    function edgeFade(y, extent) {
      var t=Math.max(0,Math.min(1,y/40,(extent-y)/40));
      return t*t*(3-2*t);
    }
    function draw() {
      if (!ctx) return;
      ctx.clearRect(0, 0, width, height);
      glows.forEach(function (glow, i) {
        ctx.save();
        ctx.translate(width * (0.15 + i * 0.24) + Math.sin(phase * 0.35 + i) * width * 0.15,
          height * (0.15 + i * 0.23) + Math.cos(phase * 0.3 + i) * height * 0.12);
        ctx.fillStyle = glow.paint;
        ctx.fillRect(-glow.radius, -glow.radius, glow.radius * 2, glow.radius * 2);
        ctx.restore();
      });
      // Layered ribbons form soft aurora curtains without full-screen blur.
      [155,195,275].forEach(function(hue, band) {
        for (var layer=10;layer>0;layer--) {
          ctx.strokeStyle='hsla('+hue+',90%,65%,'+(0.012+(10-layer)*0.003)+')';
          ctx.lineWidth=(12+layer*8)*Math.min(height/800,1.5);
          ctx.beginPath();
          for (var point=0;point<=64;point++) {
            var x=point/64*width;
            var y=height*(0.2+band*0.25+0.09*Math.sin(point/64*7+phase*.45+band*2)+0.025*Math.sin(point/64*18-phase*.65+band));
            if(point===0)ctx.moveTo(x,y);else ctx.lineTo(x,y);
          }
          ctx.stroke();
        }
      });
      stars.forEach(function (star) {
        var sy=(star.y*(height+80)+phase*star.speed*22)%(height+80)-40;
        ctx.globalAlpha = (0.55 + Math.sin(phase * 1.8 + star.x * 20) * 0.2)*edgeFade(sy,height);
        ctx.fillStyle = 'hsl(' + star.hue + ',95%,76%)';
        ctx.beginPath();
        ctx.arc(star.x * width, sy, star.r, 0, Math.PI * 2);
        ctx.fill();
      });
      // Eight staggered, colorful meteors with soft fading tails.
      for (var m = 0; m < 8; m++) {
        var travel = (phase * 0.28 + m / 8) % 1;
        var mx = (m % 2 ? 1 - travel : travel) * (width + 400) - 200;
        var my = ((m * 0.31) % 0.5 + travel * 0.5) * height;
        ctx.save();ctx.translate(mx, my);
        ctx.scale(m % 2 ? -1 : 1, 1);
        ctx.scale(0.7+(m%3)*0.3,0.7+(m%3)*0.3);
        ctx.globalAlpha = Math.pow(Math.sin(travel * Math.PI),2) * 0.8;
        var tail = ctx.createLinearGradient(-100, -48, 0, 0);
        tail.addColorStop(0, 'hsla(' + palette[m % palette.length] + ',100%,70%,0)');
        tail.addColorStop(1, 'hsl(' + palette[m % palette.length] + ',100%,85%)');
        ctx.strokeStyle = tail;ctx.lineWidth = 2;
        ctx.beginPath();ctx.moveTo(-100, -48);ctx.lineTo(0, 0);ctx.stroke();
        ctx.restore();
      }
      // Original outline storage symbols; no third-party images or fonts.
      for (var i = 0; i < 5; i++) {
        ctx.save();
        var iy=(height*(0.12+i*0.19)+phase*(12+i*1.2))%(height+120)-60;
        ctx.globalAlpha = 0.48*edgeFade(iy,height);
        ctx.strokeStyle = 'hsl(' + palette[i] + ',95%,74%)';
        ctx.lineWidth = 1.2;
        ctx.translate(width * (0.09 + i * 0.2) + Math.sin(phase / 3 + i) * 14,
          iy);
        ctx.rotate(Math.sin(phase / 5 + i) * 0.15);
        ctx.strokeRect(-17, -14, 34, 12); ctx.strokeRect(-17, 3, 34, 12);
        ctx.beginPath();ctx.moveTo(-11, -8);ctx.lineTo(-7, -8);ctx.moveTo(-11, 9);ctx.lineTo(-7, 9);ctx.stroke();
        ctx.restore();
      }
      ctx.globalAlpha = 1;
    }
    function active() { return SBPreferences.get() === 'space' && !document.hidden && !suspended && !contrast.matches && !!ctx; }
    function tick(now) {
      frame = 0;
      if (!active()) return;
      if (!last || now - last >= 1000 / 30) {
        phase += last ? Math.min((now - last) / 1000, 0.1) : 0;
        last = now; draw();
      }
      frame = requestAnimationFrame(tick);
    }
    function sync() {
      cancelAnimationFrame(frame);frame = 0;last = 0;
      canvas.hidden = SBPreferences.get() !== 'space';
      if (SBPreferences.get() === 'space' && !document.hidden && !suspended) { size();draw(); }
      if (active()) frame = requestAnimationFrame(tick);
    }
    document.addEventListener('visibilitychange', sync);
    window.addEventListener('pagehide', function () { suspended = true;sync(); });
    window.addEventListener('pageshow', function () { suspended = false;sync(); });
    window.addEventListener('resize', sync);
    if (contrast.addEventListener) contrast.addEventListener('change', sync);
    SBPreferences.subscribe(sync);sync();
})();
