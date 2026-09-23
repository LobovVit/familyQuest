/// <reference types="node" />
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'
import ts from 'typescript'
import { expect, it } from 'vitest'

const root=resolve(import.meta.dirname,'..')
const allowed:Record<string,string[]>={domain:['domain'],application:['application','domain'],infrastructure:['infrastructure','application','domain'],features:['features','application','domain'],pages:['pages','features','application','domain']}
function files(dir:string):string[]{return readdirSync(dir,{withFileTypes:true}).flatMap(e=>e.isDirectory()?files(join(dir,e.name)):/\.tsx?$/.test(e.name)&&!e.name.includes('.test.')?[join(dir,e.name)]:[])}
it('keeps production imports directed toward domain and application ports',()=>{
 const violations:string[]=[]
 for(const [layer,permitted] of Object.entries(allowed))for(const file of files(join(root,layer))){
  const ast=ts.createSourceFile(file,readFileSync(file,'utf8'),ts.ScriptTarget.Latest,true)
  function visit(node:ts.Node){
   let target:string|undefined
   if((ts.isImportDeclaration(node)||ts.isExportDeclaration(node))&&node.moduleSpecifier&&ts.isStringLiteral(node.moduleSpecifier))target=node.moduleSpecifier.text
   if(ts.isCallExpression(node)&&node.expression.kind===ts.SyntaxKind.ImportKeyword&&node.arguments[0]&&ts.isStringLiteral(node.arguments[0]))target=node.arguments[0].text
   if(ts.isImportTypeNode(node)&&ts.isLiteralTypeNode(node.argument)&&ts.isStringLiteral(node.argument.literal))target=node.argument.literal.text
   if(target && !(['features','pages'].includes(layer) && /\.(css|svg|png)$/.test(target))){
    if(target.startsWith('.')){
     const destination=relative(root,resolve(dirname(file),target)).split('/')[0]
     if(!permitted.includes(destination))violations.push(`${relative(root,file)} -> ${target}`)
    }else if(layer==='domain'||(layer==='application'&&target!=='react'))violations.push(`${relative(root,file)} -> external ${target}`)
   }
   if(['domain','application'].includes(layer)&&ts.isIdentifier(node)&&['fetch','localStorage','sessionStorage','XMLHttpRequest','WebSocket'].includes(node.text))violations.push(`${relative(root,file)} uses ${node.text}`)
   ts.forEachChild(node,visit)
  }
  visit(ast)
 }
 expect(violations).toEqual([])
})
