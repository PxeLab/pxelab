import{c,u as m,j as s}from"./index-DpcR_ZnY.js";/**
 * @license lucide-react v0.500.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const f=[["path",{d:"m15 18-6-6 6-6",key:"1wnfg3"}]],u=c("chevron-left",f);/**
 * @license lucide-react v0.500.0 - ISC
 *
 * This source code is licensed under the ISC license.
 * See the LICENSE file in the root directory of this source tree.
 */const j=[["path",{d:"m9 18 6-6-6-6",key:"mthhwq"}]],v=c("chevron-right",j),y=({page:t,total:n,size:r,onChange:o})=>{const{t:d}=m(),i=Math.ceil(n/r);if(i<=1)return null;const a=[],l=Math.max(1,t-2),b=Math.min(i,t+2);for(let e=l;e<=b;e++)a.push(e);const h=(t-1)*r+1,x=Math.min(t*r,n);return s.jsxs("div",{className:"flex items-center justify-between pt-4 text-xs text-[#6b7294]",children:[s.jsx("span",{children:d("pagination.showing",{from:h,to:x,total:n})}),s.jsxs("div",{className:"flex gap-1",children:[s.jsx("button",{onClick:()=>o(t-1),disabled:t<=1,className:"w-7 h-7 flex items-center justify-center rounded border border-[#232738] bg-transparent text-[#9aa0ab] disabled:opacity-40 hover:bg-[#16181f] transition-colors",children:s.jsx(u,{size:14})}),a.map(e=>s.jsx("button",{onClick:()=>o(e),className:`w-7 h-7 flex items-center justify-center rounded border text-xs font-mono transition-colors ${e===t?"bg-blue-500 border-blue-500 text-white":"border-[#232738] bg-transparent text-[#9aa0ab] hover:bg-[#16181f]"}`,children:e},e)),s.jsx("button",{onClick:()=>o(t+1),disabled:t>=i,className:"w-7 h-7 flex items-center justify-center rounded border border-[#232738] bg-transparent text-[#9aa0ab] disabled:opacity-40 hover:bg-[#16181f] transition-colors",children:s.jsx(v,{size:14})})]})]})};export{y as P};
