// KeepSave Orbit icon family. Original 24-unit vector artwork; see docs/design/2026-09-28-icon-family.
export type IconNode = readonly [tag: 'path' | 'circle' | 'rect' | 'ellipse', attrs: Record<string, string | number>];
export const artwork = {
  "FolderClosed": [
    [
      "path",
      {
        "d": "M3 8V6.5A2 2 0 0 1 5 4.5h4l2.5 3H19a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8Z"
      }
    ],
    [
      "ellipse",
      {
        "cx": 12,
        "cy": 14,
        "rx": 4.5,
        "ry": 2.2,
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 14,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "FolderOpen": [
    [
      "path",
      {
        "d": "M3.5 9V6.5a2 2 0 0 1 2-2H9l2.5 3H19M3.5 10h17l-2 10h-15l-1-8a2 2 0 0 1 1-2Z"
      }
    ],
    [
      "ellipse",
      {
        "cx": 12,
        "cy": 15,
        "rx": 3.8,
        "ry": 1.7,
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 15,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "FolderKey": [
    [
      "path",
      {
        "d": "M3 8V6.5A2 2 0 0 1 5 4.5h4l2.5 3H19a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8Z"
      }
    ],
    [
      "circle",
      {
        "cx": 10,
        "cy": 13,
        "r": 2.1
      }
    ],
    [
      "path",
      {
        "d": "m11.5 14.5 4 3m-1.5-1.1 1.2-1.5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Building2": [
    [
      "path",
      {
        "d": "M4 20V8a1 1 0 0 1 1-1h5V4a1 1 0 0 1 1-1h8a1 1 0 0 1 1 1v16M2 20h20M7 11v1m0 3v1M13 20v-4h4v4"
      }
    ],
    [
      "circle",
      {
        "cx": 15,
        "cy": 8,
        "r": 2.2
      }
    ],
    [
      "path",
      {
        "d": "M12 11c1.5 1 4.5 1 6 0",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Boxes": [
    [
      "path",
      {
        "d": "M3 15V5a2 2 0 0 1 2-2h10M6.5 18V8.5a2 2 0 0 1 2-2H18"
      }
    ],
    [
      "rect",
      {
        "x": 10,
        "y": 10,
        "width": 11,
        "height": 11,
        "rx": 2.5
      }
    ],
    [
      "circle",
      {
        "cx": 15.5,
        "cy": 15.5,
        "r": 2.4
      }
    ],
    [
      "path",
      {
        "d": "M11.5 17c2.5 1.2 5.5.8 8-2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "AppWindow": [
    [
      "rect",
      {
        "x": 3,
        "y": 4,
        "width": 18,
        "height": 16,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M3 9h18M6.5 6.5h.1m3 0h.1"
      }
    ],
    [
      "ellipse",
      {
        "cx": 12,
        "cy": 14.5,
        "rx": 4.3,
        "ry": 2,
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 14.5,
        "r": 1.1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Server": [
    [
      "rect",
      {
        "x": 4,
        "y": 3,
        "width": 16,
        "height": 8,
        "rx": 2.5
      }
    ],
    [
      "rect",
      {
        "x": 4,
        "y": 13,
        "width": 16,
        "height": 8,
        "rx": 2.5
      }
    ],
    [
      "circle",
      {
        "cx": 8,
        "cy": 7,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 8,
        "cy": 17,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "M12 7h4M12 17h4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "OrbitHub": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 3.4
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "M6.4 7.4a8 8 0 0 1 11.2 0M19.4 13a8 8 0 0 1-5.8 6.8M9.2 19.2A8 8 0 0 1 4.6 10",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 4.5,
        "cy": 7,
        "r": 1.8
      }
    ],
    [
      "circle",
      {
        "cx": 19.5,
        "cy": 9,
        "r": 1.8
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 20,
        "r": 1.8
      }
    ]
  ],
  "Cable": [
    [
      "path",
      {
        "d": "M6.5 3v4m-3-4v4m13-4v4m-3-4v4M2 7h6v3a3 3 0 0 1-6 0V7Zm10 0h6v3a3 3 0 0 1-6 0V7ZM5 13v4a4 4 0 0 0 8 0v-1M15 13v4a4 4 0 0 0 7 2"
      }
    ],
    [
      "circle",
      {
        "cx": 22,
        "cy": 15,
        "r": 1.1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Bot": [
    [
      "rect",
      {
        "x": 4,
        "y": 7,
        "width": 16,
        "height": 13,
        "rx": 4
      }
    ],
    [
      "path",
      {
        "d": "M12 7V4M2 12v3m20-3v3M9 17h6"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 3,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 8.5,
        "cy": 12,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 15.5,
        "cy": 12,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "M6 9.5c3-2 9-2 12 0",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Brain": [
    [
      "path",
      {
        "d": "M12 5c-1-3-6-2-6 2-3 0-4 5-1 7-1 4 3 7 7 4m0-13c1-3 6-2 6 2 3 0 4 5 1 7 1 4-3 7-7 4M12 5v14"
      }
    ],
    [
      "path",
      {
        "d": "M7 10c3-3 7-3 10 0M7 15c3 3 7 3 10 0",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12.5,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "BookOpen": [
    [
      "path",
      {
        "d": "M12 6C9 4 6 3.5 3 4v15c3-.5 6 0 9 2 3-2 6-2.5 9-2V4c-3-.5-6 0-9 2Zm0 0v15M6 9c1.2 0 2.2.2 3 .7M6 13c1.2 0 2.2.2 3 .7"
      }
    ],
    [
      "path",
      {
        "d": "M15 9c1.2-.5 2.2-.7 3-.7m-3 4.7c1.2-.5 2.2-.7 3-.7",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "LockKeyhole": [
    [
      "rect",
      {
        "x": 5,
        "y": 10,
        "width": 14,
        "height": 11,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M8 10V7a4 4 0 0 1 8 0v3"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 14.5,
        "r": 1.3
      }
    ],
    [
      "path",
      {
        "d": "M12 16v2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Lock": [
    [
      "rect",
      {
        "x": 5,
        "y": 10,
        "width": 14,
        "height": 11,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M8 10V7a4 4 0 0 1 8 0v3"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 14.5,
        "r": 1.3
      }
    ],
    [
      "path",
      {
        "d": "M12 16v2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "KeyRound": [
    [
      "circle",
      {
        "cx": 8,
        "cy": 8,
        "r": 5
      }
    ],
    [
      "circle",
      {
        "cx": 8,
        "cy": 8,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "m11.6 11.6 8.4 8.4m-3-3 2.5-2.5m-5 0 2.5-2.5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Key": [
    [
      "circle",
      {
        "cx": 8,
        "cy": 8,
        "r": 5
      }
    ],
    [
      "circle",
      {
        "cx": 8,
        "cy": 8,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "m11.6 11.6 8.4 8.4m-3-3 2.5-2.5m-5 0 2.5-2.5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Shield": [
    [
      "path",
      {
        "d": "M12 3c2.5 2 5 2.6 8 3v6c0 4.5-3.3 7.5-8 9-4.7-1.5-8-4.5-8-9V6c3-.4 5.5-1 8-3Z"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 11.5,
        "r": 3
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 11.5,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "ShieldCheck": [
    [
      "path",
      {
        "d": "M12 3c2.5 2 5 2.6 8 3v6c0 4.5-3.3 7.5-8 9-4.7-1.5-8-4.5-8-9V6c3-.4 5.5-1 8-3Z"
      }
    ],
    [
      "path",
      {
        "d": "m8.5 11.5 2.5 2.5 4.8-5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "ShieldAlert": [
    [
      "path",
      {
        "d": "M12 3c2.5 2 5 2.6 8 3v6c0 4.5-3.3 7.5-8 9-4.7-1.5-8-4.5-8-9V6c3-.4 5.5-1 8-3Z"
      }
    ],
    [
      "path",
      {
        "d": "M12 8v5",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 16,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Search": [
    [
      "circle",
      {
        "cx": 10,
        "cy": 10,
        "r": 6
      }
    ],
    [
      "path",
      {
        "d": "m14.5 14.5 6 6",
        "className": "ks-icon-detail"
      }
    ],
    [
      "path",
      {
        "d": "M7 6.8a4.5 4.5 0 0 1 5-.5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Plus": [
    [
      "path",
      {
        "d": "M5 12h14M12 5v14"
      }
    ]
  ],
  "X": [
    [
      "path",
      {
        "d": "m6 6 12 12M18 6 6 18"
      }
    ]
  ],
  "Check": [
    [
      "path",
      {
        "d": "m4.5 12 5 5 10-11"
      }
    ]
  ],
  "Trash2": [
    [
      "path",
      {
        "d": "M4 7c4-1.3 12-1.3 16 0M9 5V3h6v2M6 8l1 11a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-11"
      }
    ],
    [
      "path",
      {
        "d": "M10 10.5v6.5m4-6.5v6.5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Copy": [
    [
      "rect",
      {
        "x": 8,
        "y": 8,
        "width": 12,
        "height": 13,
        "rx": 2.5
      }
    ],
    [
      "path",
      {
        "d": "M5 16H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h11a1 1 0 0 1 1 1v1",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Pencil": [
    [
      "path",
      {
        "d": "m4 15 11-11a2.1 2.1 0 0 1 3 0l2 2a2.1 2.1 0 0 1 0 3L9 20l-6 1 1-6Z"
      }
    ],
    [
      "path",
      {
        "d": "m13 6 5 5M4 15l5 5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Eye": [
    [
      "path",
      {
        "d": "M2 12c2.5-4.5 6-7 10-7s7.5 2.5 10 7c-2.5 4.5-6 7-10 7S4.5 16.5 2 12Z"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 3.2
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "EyeOff": [
    [
      "path",
      {
        "d": "M3 3l18 18M9.5 5.3A12 12 0 0 1 12 5c4 0 7.5 2.5 10 7a18 18 0 0 1-3 4M6 6.7A19 19 0 0 0 2 12c2.5 4.5 6 7 10 7a11 11 0 0 0 4-1"
      }
    ],
    [
      "path",
      {
        "d": "M9 10a3.2 3.2 0 0 0 5 4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Upload": [
    [
      "path",
      {
        "d": "M12 16V3m-5 5 5-5 5 5"
      }
    ],
    [
      "path",
      {
        "d": "M4 16v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Download": [
    [
      "path",
      {
        "d": "M12 3v13m-5-5 5 5 5-5"
      }
    ],
    [
      "path",
      {
        "d": "M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "ArrowRight": [
    [
      "path",
      {
        "d": "M3 12h17M14 6l6 6-6 6"
      }
    ]
  ],
  "ArrowLeft": [
    [
      "path",
      {
        "d": "M21 12H4m6-6-6 6 6 6"
      }
    ]
  ],
  "ArrowDown": [
    [
      "path",
      {
        "d": "M12 3v17m-6-6 6 6 6-6"
      }
    ]
  ],
  "ArrowUpRight": [
    [
      "path",
      {
        "d": "M5 19 19 5M9 5h10v10"
      }
    ]
  ],
  "ChevronDown": [
    [
      "path",
      {
        "d": "m6 9 6 6 6-6"
      }
    ]
  ],
  "ChevronUp": [
    [
      "path",
      {
        "d": "m6 15 6-6 6 6"
      }
    ]
  ],
  "ChevronRight": [
    [
      "path",
      {
        "d": "m9 6 6 6-6 6"
      }
    ]
  ],
  "CornerDownLeft": [
    [
      "path",
      {
        "d": "M20 4v8a3 3 0 0 1-3 3H4m5-5-5 5 5 5"
      }
    ]
  ],
  "LogOut": [
    [
      "path",
      {
        "d": "M10 3H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h5"
      }
    ],
    [
      "path",
      {
        "d": "M8 12h13m-5-5 5 5-5 5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Menu": [
    [
      "path",
      {
        "d": "M4 6h16M4 12h12M4 18h16"
      }
    ]
  ],
  "List": [
    [
      "path",
      {
        "d": "M9 5h12M9 12h12M9 19h12"
      }
    ],
    [
      "circle",
      {
        "cx": 4,
        "cy": 5,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 4,
        "cy": 12,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 4,
        "cy": 19,
        "r": 1.3,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "LayoutGrid": [
    [
      "rect",
      {
        "x": 3,
        "y": 3,
        "width": 7,
        "height": 7,
        "rx": 2
      }
    ],
    [
      "rect",
      {
        "x": 14,
        "y": 3,
        "width": 7,
        "height": 7,
        "rx": 2
      }
    ],
    [
      "rect",
      {
        "x": 3,
        "y": 14,
        "width": 7,
        "height": 7,
        "rx": 2
      }
    ],
    [
      "rect",
      {
        "x": 14,
        "y": 14,
        "width": 7,
        "height": 7,
        "rx": 2
      }
    ]
  ],
  "LayoutDashboard": [
    [
      "rect",
      {
        "x": 3,
        "y": 3,
        "width": 7,
        "height": 11,
        "rx": 2
      }
    ],
    [
      "rect",
      {
        "x": 14,
        "y": 3,
        "width": 7,
        "height": 6,
        "rx": 2
      }
    ],
    [
      "rect",
      {
        "x": 3,
        "y": 18,
        "width": 7,
        "height": 3,
        "rx": 1.5
      }
    ],
    [
      "rect",
      {
        "x": 14,
        "y": 13,
        "width": 7,
        "height": 8,
        "rx": 2
      }
    ]
  ],
  "PanelLeftClose": [
    [
      "rect",
      {
        "x": 3,
        "y": 4,
        "width": 18,
        "height": 16,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M9 4v16m7-12-4 4 4 4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "PanelLeftOpen": [
    [
      "rect",
      {
        "x": 3,
        "y": 4,
        "width": 18,
        "height": 16,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M9 4v16m4-12 4 4-4 4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Home": [
    [
      "path",
      {
        "d": "m3 11 9-8 9 8M5 9v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V9M10 21v-7h4v7"
      }
    ]
  ],
  "CheckCircle2": [
    [
      "path",
      {
        "d": "M20.8 10A9 9 0 1 1 16 3.9"
      }
    ],
    [
      "path",
      {
        "d": "m8 10 4 4L21 4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "CheckCircle": [
    [
      "path",
      {
        "d": "M20.8 10A9 9 0 1 1 16 3.9"
      }
    ],
    [
      "path",
      {
        "d": "m8 10 4 4L21 4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "XCircle": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 9
      }
    ],
    [
      "path",
      {
        "d": "m8.5 8.5 7 7m0-7-7 7",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "AlertCircle": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 9
      }
    ],
    [
      "path",
      {
        "d": "M12 7v6",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 17,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "AlertTriangle": [
    [
      "path",
      {
        "d": "M10.5 3.7a1.7 1.7 0 0 1 3 0l8 14.5a1.7 1.7 0 0 1-1.5 2.5H4a1.7 1.7 0 0 1-1.5-2.5l8-14.5Z"
      }
    ],
    [
      "path",
      {
        "d": "M12 8v5",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 17,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "AlertOctagon": [
    [
      "path",
      {
        "d": "M8 3h8l5 5v8l-5 5H8l-5-5V8l5-5Z"
      }
    ],
    [
      "path",
      {
        "d": "M12 7v6",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 17,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Info": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 9
      }
    ],
    [
      "path",
      {
        "d": "M12 11v6",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 7,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Ban": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 9
      }
    ],
    [
      "path",
      {
        "d": "m6 6 12 12",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Clock": [
    [
      "path",
      {
        "d": "M20.8 10A9 9 0 1 1 14 3.2"
      }
    ],
    [
      "path",
      {
        "d": "M12 7v5l4 2",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 18.3,
        "cy": 5.5,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "History": [
    [
      "path",
      {
        "d": "M4 9a8 8 0 1 1 0 6M3 4v5h5"
      }
    ],
    [
      "path",
      {
        "d": "M12 7v5l4 2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Timer": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 14,
        "r": 7.5
      }
    ],
    [
      "path",
      {
        "d": "M12 6.5V3M9 2h6m2 5 2-2M12 10v4l3 2",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "RefreshCw": [
    [
      "path",
      {
        "d": "M20 8a8 8 0 0 0-14-3L3 8m0-5v5h5M4 16a8 8 0 0 0 14 3l3-3m0 5v-5h-5"
      }
    ]
  ],
  "RotateCcw": [
    [
      "path",
      {
        "d": "M4 9a8 8 0 1 1 0 6M3 4v5h5"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1.2,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "LoaderCircle": [
    [
      "path",
      {
        "d": "M20.6 14.5A9 9 0 1 1 14.5 3.4"
      }
    ],
    [
      "circle",
      {
        "cx": 19,
        "cy": 6,
        "r": 1.4,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Activity": [
    [
      "path",
      {
        "d": "M2 12h5l3-7 4 14 3-7h5"
      }
    ]
  ],
  "Radio": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1.6,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "path",
      {
        "d": "M8.5 8.5a5 5 0 0 0 0 7m7-7a5 5 0 0 1 0 7M5 5a10 10 0 0 0 0 14M19 5a10 10 0 0 1 0 14",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Gauge": [
    [
      "path",
      {
        "d": "M4 19a10 10 0 1 1 16 0H4ZM5 12h2m10 0h2M12 4v2"
      }
    ],
    [
      "path",
      {
        "d": "m12 15 5-7",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 15,
        "r": 1.4,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Sun": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 4
      }
    ],
    [
      "path",
      {
        "d": "M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1.5 1.5m11 11L19 19M5 19l1.5-1.5m11-11L19 5",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Moon": [
    [
      "path",
      {
        "d": "M19.8 14.5A8.8 8.8 0 0 1 9.5 4.2 9 9 0 1 0 19.8 14.5Z"
      }
    ],
    [
      "path",
      {
        "d": "M14 4c2 0 4 2 4 4",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 16,
        "cy": 6,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "GitBranch": [
    [
      "circle",
      {
        "cx": 6,
        "cy": 5,
        "r": 2
      }
    ],
    [
      "circle",
      {
        "cx": 6,
        "cy": 19,
        "r": 2
      }
    ],
    [
      "circle",
      {
        "cx": 18,
        "cy": 5,
        "r": 2
      }
    ],
    [
      "path",
      {
        "d": "M6 7v10m12-10v2c0 6-12 1-12 8",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "GitPullRequest": [
    [
      "circle",
      {
        "cx": 6,
        "cy": 5,
        "r": 2
      }
    ],
    [
      "circle",
      {
        "cx": 6,
        "cy": 19,
        "r": 2
      }
    ],
    [
      "circle",
      {
        "cx": 18,
        "cy": 19,
        "r": 2
      }
    ],
    [
      "path",
      {
        "d": "M6 7v10m12 0V9a4 4 0 0 0-4-4h-2m3-3-3 3 3 3",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "GitCompare": [
    [
      "circle",
      {
        "cx": 6,
        "cy": 4,
        "r": 1.8
      }
    ],
    [
      "circle",
      {
        "cx": 18,
        "cy": 20,
        "r": 1.8
      }
    ],
    [
      "path",
      {
        "d": "M6 6v10h5m-3-3 3 3-3 3M18 18V8h-5m3-3-3 3 3 3",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Layers": [
    [
      "path",
      {
        "d": "m3 8 9-5 9 5-9 5-9-5Zm0 5 9 5 9-5m-18 5 9 5 9-5"
      }
    ]
  ],
  "BarChart3": [
    [
      "path",
      {
        "d": "M4 4v16h17"
      }
    ],
    [
      "path",
      {
        "d": "M8 15v-4m5 4V6m5 9V9",
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 13,
        "cy": 4,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "TrendingUp": [
    [
      "path",
      {
        "d": "m3 17 6-6 4 3 8-10M15 4h6v6"
      }
    ]
  ],
  "TrendingDown": [
    [
      "path",
      {
        "d": "m3 7 6 6 4-3 8 10m-6 0h6v-6"
      }
    ]
  ],
  "Calendar": [
    [
      "rect",
      {
        "x": 3,
        "y": 5,
        "width": 18,
        "height": 16,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "M7 3v4m10-4v4M3 10h18"
      }
    ],
    [
      "circle",
      {
        "cx": 8,
        "cy": 15,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 15,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ],
    [
      "circle",
      {
        "cx": 16,
        "cy": 15,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Code2": [
    [
      "path",
      {
        "d": "m7 7-5 5 5 5m10-10 5 5-5 5"
      }
    ],
    [
      "path",
      {
        "d": "m14 4-4 16",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "FileCode2": [
    [
      "path",
      {
        "d": "M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9l-6-6Zm0 0v6h6"
      }
    ],
    [
      "path",
      {
        "d": "m9 12-3 3 3 3m6-6 3 3-3 3",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Terminal": [
    [
      "rect",
      {
        "x": 3,
        "y": 4,
        "width": 18,
        "height": 16,
        "rx": 3
      }
    ],
    [
      "path",
      {
        "d": "m7 9 3 3-3 3m6 1h4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "UserRound": [
    [
      "circle",
      {
        "cx": 12,
        "cy": 7,
        "r": 3.5
      }
    ],
    [
      "path",
      {
        "d": "M4 21v-2a8 8 0 0 1 16 0v2"
      }
    ],
    [
      "path",
      {
        "d": "M5 18c4 2 10 2 14 0",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Users": [
    [
      "circle",
      {
        "cx": 9,
        "cy": 7,
        "r": 3
      }
    ],
    [
      "path",
      {
        "d": "M2 21v-2a7 7 0 0 1 14 0v2m0-17a3 3 0 0 1 0 6m3 4a6 6 0 0 1 3 5v2"
      }
    ],
    [
      "path",
      {
        "d": "M4 18c3 1.5 7 1.5 10 0",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "MessageSquare": [
    [
      "path",
      {
        "d": "M5 4h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9l-6 4V6a2 2 0 0 1 2-2Z"
      }
    ],
    [
      "path",
      {
        "d": "M7 9h10m-10 4h6",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "MessageCircle": [
    [
      "path",
      {
        "d": "M3.8 17.2A9 9 0 1 1 8 20.2L3 22l.8-4.8Z"
      }
    ],
    [
      "ellipse",
      {
        "cx": 12,
        "cy": 11,
        "rx": 4.4,
        "ry": 2.2,
        "className": "ks-icon-detail"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 11,
        "r": 1.1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "MessageSquarePlus": [
    [
      "path",
      {
        "d": "M5 4h14a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H9l-6 4V6a2 2 0 0 1 2-2Z"
      }
    ],
    [
      "path",
      {
        "d": "M8 11h8m-4-4v8",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Send": [
    [
      "path",
      {
        "d": "m3 3 19 9-19 9 4-9-4-9Z"
      }
    ],
    [
      "path",
      {
        "d": "M7 12h15",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Lightbulb": [
    [
      "path",
      {
        "d": "M8 16a7 7 0 1 1 8 0v3H8v-3ZM9 22h6"
      }
    ],
    [
      "path",
      {
        "d": "M8 10c2 1.5 6 1.5 8 0M12 12v4",
        "className": "ks-icon-detail"
      }
    ]
  ],
  "Settings": [
    [
      "path",
      {
        "d": "m9 3-1 3-3 1-2 4 2 2v3l3 3 3-1 3 3 4-2v-3l3-2-1-4-3-1-1-4h-4l-3-2Z"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 3.5
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Wrench": [
    [
      "path",
      {
        "d": "M21 4a6 6 0 0 1-8 8L5 21l-3-3 9-8a6 6 0 0 1 8-8l-4 4 3 3 3-5Z"
      }
    ]
  ],
  "Puzzle": [
    [
      "path",
      {
        "d": "M4 4h5a3 3 0 1 1 6 0h5v5a3 3 0 1 0 0 6v5h-5a3 3 0 1 0-6 0H4v-5a3 3 0 1 1 0-6V4Z"
      }
    ]
  ],
  "Star": [
    [
      "path",
      {
        "d": "m12 3 2.8 5.8 6.4 1-4.6 4.5 1.1 6.4-5.7-3-5.7 3 1.1-6.4-4.6-4.5 6.4-1L12 3Z"
      }
    ],
    [
      "circle",
      {
        "cx": 12,
        "cy": 12,
        "r": 1,
        "fill": "currentColor",
        "stroke": "none",
        "className": "ks-icon-core"
      }
    ]
  ],
  "Power": [
    [
      "path",
      {
        "d": "M12 2v10M7 5a9 9 0 1 0 10 0"
      }
    ]
  ],
  "PowerOff": [
    [
      "path",
      {
        "d": "M12 2v7M7 5a9 9 0 0 0 10 15M17 5a9 9 0 0 1 4 11M3 3l18 18"
      }
    ]
  ],
  "Pause": [
    [
      "rect",
      {
        "x": 5,
        "y": 4,
        "width": 4,
        "height": 16,
        "rx": 1.5
      }
    ],
    [
      "rect",
      {
        "x": 15,
        "y": 4,
        "width": 4,
        "height": 16,
        "rx": 1.5
      }
    ]
  ],
  "Play": [
    [
      "path",
      {
        "d": "M6 3.5 21 12 6 20.5v-17Z"
      }
    ]
  ]
} satisfies Record<string, IconNode[]>;
export type IconName = keyof typeof artwork;
