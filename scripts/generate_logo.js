const fs = require('fs');
const path = require('path');
const { Resvg } = require('@resvg/resvg-js');

function buildLogoSvg() {
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1024 1024" width="1024" height="1024">
  <defs>
    <clipPath id="squircle-clip">
      <rect x="24" y="24" width="976" height="976" rx="220" />
    </clipPath>

    <!-- Optics Gradient: Electric Cyan to Deep Cerulean -->
    <linearGradient id="cyan-optics" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#00f5ff"/>
      <stop offset="100%" stop-color="#0284c7"/>
    </linearGradient>

    <!-- Beak Metallic Gradient: Refined Slate Silver to Obsidian -->
    <linearGradient id="beak-metallic-left" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#f8fafc"/>
      <stop offset="100%" stop-color="#cbd5e1"/>
    </linearGradient>
    <linearGradient id="beak-metallic-right" x1="100%" y1="0%" x2="0%" y2="100%">
      <stop offset="0%" stop-color="#94a3b8"/>
      <stop offset="100%" stop-color="#64748b"/>
    </linearGradient>

    <!-- Eye Socket Depth Filter -->
    <filter id="eye-glow" x="-20%" y="-20%" width="140%" height="140%">
      <feDropShadow dx="0" dy="0" stdDeviation="6" flood-color="#00f5ff" flood-opacity="0.6"/>
    </filter>

    <filter id="subtle-shadow" x="-10%" y="-10%" width="120%" height="120%">
      <feDropShadow dx="0" dy="16" stdDeviation="20" flood-color="#000000" flood-opacity="0.14" />
    </filter>
  </defs>

  <!-- Luxury White Squircle Container -->
  <rect x="24" y="24" width="976" height="976" rx="220" fill="#ffffff" stroke="#e2e8f0" stroke-width="6" />

  <g clip-path="url(#squircle-clip)">
    <g transform="translate(512, 512)" filter="url(#subtle-shadow)">

      <!-- Massive Hexagonal Architectural Gateway Frame -->
      <polygon points="
        0,-390
        338,-195
        338,195
        0,390
        -338,195
        -338,-195
      " fill="none" stroke="#0f172a" stroke-width="40" stroke-linejoin="round" />

      <!-- Inner Precision Radar Perimeter (Telemetry Ring) -->
      <polygon points="
        0,-355
        307,-177
        307,177
        0,355
        -307,177
        -307,-177
      " fill="none" stroke="#00f5ff" stroke-width="4" opacity="0.4" stroke-dasharray="16, 12" />

      <!-- Corner Alignment Crosshairs -->
      <line x1="-338" y1="-195" x2="-307" y2="-177" stroke="#00f5ff" stroke-width="3" opacity="0.5" />
      <line x1="338" y1="-195" x2="307" y2="-177" stroke="#00f5ff" stroke-width="3" opacity="0.5" />
      <line x1="338" y1="195" x2="307" y2="177" stroke="#00f5ff" stroke-width="3" opacity="0.5" />
      <line x1="-338" y1="195" x2="-307" y2="177" stroke="#00f5ff" stroke-width="3" opacity="0.5" />
      <line x1="0" y1="-390" x2="0" y2="-355" stroke="#00f5ff" stroke-width="3" opacity="0.5" />
      <line x1="0" y1="390" x2="0" y2="355" stroke="#00f5ff" stroke-width="3" opacity="0.5" />

      <!-- ======================================================== -->
      <!-- THE SHADOW OWL (ATHENE) - 85% MASS SYMMETRICAL ORIGAMI   -->
      <!-- ======================================================== -->

      <!-- 1. Solid Watertight Under-Silhouette (Zero Vertex Leakage) -->
      <path d="
        M 0,-215
        L 85,-245 L 165,-285 L 195,-220 L 225,-150 L 255,-50 L 275,50
        L 285,165 L 250,270 L 190,335 L 115,365 L 0,380
        L -115,365 L -190,335 L -250,270 L -285,165 L -275,50
        L -255,-50 L -225,-150 L -195,-220 L -165,-285 L -85,-245
        Z
      " fill="#0b0f19" />

      <!-- 2. Wings & Shoulders (Outer Facets) -->
      <!-- Left Outer Wing -->
      <polygon points="-275,50 -285,165 -220,195 -190,100" fill="#0f172a" />
      <polygon points="-285,165 -250,270 -185,255 -220,195" fill="#1e293b" />
      <polygon points="-250,270 -190,335 -135,290 -185,255" fill="#0f172a" />
      <polygon points="-190,335 -115,365 -75,320 -135,290" fill="#1e293b" />

      <!-- Right Outer Wing (Symmetrical) -->
      <polygon points="275,50 285,165 220,195 190,100" fill="#1e293b" />
      <polygon points="285,165 250,270 185,255 220,195" fill="#334155" />
      <polygon points="250,270 190,335 135,290 185,255" fill="#1e293b" />
      <polygon points="190,335 115,365 75,320 135,290" fill="#334155" />

      <!-- Lower Tail Feathers (Keel) -->
      <polygon points="0,380 -115,365 -75,320 0,335" fill="#0f172a" />
      <polygon points="0,380 115,365 75,320 0,335" fill="#1e293b" />

      <!-- 3. Abdomen & Thoracic Core Plates -->
      <polygon points="0,210 -110,210 -135,290 -75,320 0,335" fill="#1e293b" />
      <polygon points="0,210 110,210 135,290 75,320 0,335" fill="#334155" />

      <!-- Upper Chest Breastplates -->
      <polygon points="0,115 -145,115 -190,100 -220,195 -110,210 0,210" fill="#334155" />
      <polygon points="0,115 145,115 190,100 220,195 110,210 0,210" fill="#475569" />

      <polygon points="0,75 -125,75 -145,115 0,115" fill="#1e293b" />
      <polygon points="0,75 125,75 145,115 0,115" fill="#334155" />

      <!-- 4. Ear Tufts & Crest Horns -->
      <!-- Left Ear Horn -->
      <polygon points="0,-215 -85,-245 -165,-285 -145,-190" fill="#1e293b" />
      <polygon points="-165,-285 -195,-220 -145,-190" fill="#0f172a" />
      <polygon points="-195,-220 -225,-150 -160,-135 -145,-190" fill="#1e293b" />

      <!-- Right Ear Horn (Symmetrical) -->
      <polygon points="0,-215 85,-245 165,-285 145,-190" fill="#334155" />
      <polygon points="165,-285 195,-220 145,-190" fill="#1e293b" />
      <polygon points="195,-220 225,-150 160,-135 145,-190" fill="#334155" />

      <!-- Crown Dip -->
      <polygon points="0,-215 -85,-245 -65,-175 0,-170" fill="#0f172a" />
      <polygon points="0,-215 85,-245 65,-175 0,-170" fill="#1e293b" />

      <!-- 5. Cheeks & Outer Facial Disk -->
      <!-- Left Cheeks -->
      <polygon points="-225,-150 -255,-50 -185,-35 -160,-135" fill="#0f172a" />
      <polygon points="-255,-50 -275,50 -185,55 -185,-35" fill="#1e293b" />
      <polygon points="-185,55 -125,75 0,75 0,55 -95,20 -185,-35" fill="#0f172a" />

      <!-- Right Cheeks -->
      <polygon points="225,-150 255,-50 185,-35 160,-135" fill="#334155" />
      <polygon points="255,-50 275,50 185,55 185,-35" fill="#475569" />
      <polygon points="185,55 125,75 0,75 0,55 95,20 185,-35" fill="#1e293b" />

      <!-- 6. Inner Heart-Shaped Facial Mask (Sensory Disk) -->
      <!-- Left Brow -->
      <polygon points="0,-170 -65,-175 -145,-190 -160,-135 -105,-115 0,-105" fill="#1e293b" />
      <polygon points="0,-105 -105,-115 -160,-135 -135,-65 -45,-55" fill="#334155" />

      <!-- Right Brow -->
      <polygon points="0,-170 65,-175 145,-190 160,-135 105,-115 0,-105" fill="#334155" />
      <polygon points="0,-105 105,-115 160,-135 135,-65 45,-55" fill="#475569" />

      <!-- Eye Sockets (Deep Recessed Obsidian Cavity) -->
      <polygon points="-45,-55 -135,-65 -155,-15 -95,-10 -35,-20" fill="#070a10" />
      <polygon points="45,-55 135,-65 155,-15 95,-10 35,-20" fill="#070a10" />

      <!-- Lower Orbital Rim -->
      <polygon points="-35,-20 -95,-10 -155,-15 -185,-35 -95,20 0,35" fill="#1e293b" />
      <polygon points="35,-20 95,-10 155,-15 185,-35 95,20 0,35" fill="#334155" />

      <!-- 7. Predatory Optics (Electric Cyan Slanted Almond Eyes) -->
      <!-- Left Eye Orbit & Iris -->
      <g filter="url(#eye-glow)">
        <!-- Almond Eyeball Base -->
        <polygon points="-125,-48 -60,-38 -50,-24 -115,-34" fill="url(#cyan-optics)" />
        <!-- Sharp Focused Slit Pupil -->
        <polygon points="-88,-44 -82,-43 -84,-28 -90,-29" fill="#070a10" />
        <!-- Micro Optical Specular Catchlight -->
        <polygon points="-110,-42 -102,-41 -105,-37 -113,-38" fill="#ffffff" />
      </g>

      <!-- Right Eye Orbit & Iris (Symmetrical) -->
      <g filter="url(#eye-glow)">
        <!-- Almond Eyeball Base -->
        <polygon points="125,-48 60,-38 50,-24 115,-34" fill="url(#cyan-optics)" />
        <!-- Sharp Focused Slit Pupil -->
        <polygon points="88,-44 82,-43 84,-28 90,-29" fill="#070a10" />
        <!-- Micro Optical Specular Catchlight -->
        <polygon points="102,-41 110,-42 113,-38 105,-37" fill="#ffffff" />
      </g>

      <!-- 8. Geometric Titanium Beak (Zero Buck Teeth Illusion) -->
      <!-- Left Beak Facet -->
      <polygon points="0,-70 -26,-15 0,55" fill="url(#beak-metallic-left)" stroke="#0f172a" stroke-width="1.5" />
      <!-- Right Beak Facet -->
      <polygon points="0,-70 26,-15 0,55" fill="url(#beak-metallic-right)" stroke="#0f172a" stroke-width="1.5" />
      <!-- Central Ridge -->
      <line x1="0" y1="-70" x2="0" y2="55" stroke="#ffffff" stroke-width="2" opacity="0.6" />

      <!-- Subtle Cyan Telemetry Vector from Beak to Gateway -->
      <line x1="0" y1="55" x2="0" y2="115" stroke="#00f5ff" stroke-width="2" opacity="0.4" stroke-dasharray="4, 4" />

    </g>
  </g>
</svg>`;
}

async function renderLogo(outputDir) {
  const svg = buildLogoSvg();
  const svgPath = path.join(outputDir, 'logo.svg');
  const pngPath = path.join(outputDir, 'logo.png');

  fs.writeFileSync(svgPath, svg);
  const resvg = new Resvg(svg, { fitTo: { mode: 'width', value: 1024 } });
  fs.writeFileSync(pngPath, resvg.render().asPng());
  console.log('✓ Successfully rendered logo.svg and logo.png at 1024x1024');
}

const targetDir = path.resolve(__dirname, '../docs/images');
renderLogo(targetDir).catch(err => {
  console.error(err);
  process.exit(1);
});
