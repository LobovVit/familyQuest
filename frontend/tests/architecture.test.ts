import { describe, expect, it } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { join, resolve } from 'node:path'
import ts from 'typescript'

describe('dependency direction', () => {
 for (const layer of ['domain', 'application']) {
  it(`${layer} has no outward imports`, () => {
   const dir = resolve('src', layer)
   for (const name of readdirSync(dir)) {
    if (!/\.tsx?$/.test(name) || /\.test\./.test(name)) continue
    const source = ts.createSourceFile(name, readFileSync(join(dir, name), 'utf8'), ts.ScriptTarget.Latest)
    source.forEachChild(node => {
     if (!ts.isImportDeclaration(node) || !ts.isStringLiteral(node.moduleSpecifier)) return
     const path = node.moduleSpecifier.text
     expect(path, `${layer}/${name}`).not.toMatch(/infrastructure|features|pages/)
     if (layer === 'domain') expect(path, name).not.toMatch(/application|react/)
    })
   }
  })
 }
})
