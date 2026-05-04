#!/usr/bin/env python3
"""Замена текстовых формул на формулы Word (OMML) в пояснительной записке."""

from docx import Document
from docx.shared import Pt, Cm
from docx.enum.text import WD_ALIGN_PARAGRAPH
from docx.oxml.ns import qn
from lxml import etree
import os
import copy

path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'Пояснительная_записка.docx')
doc = Document(path)

# Namespace для Office Math (OMML)
MATH_NS = 'http://schemas.openxmlformats.org/officeDocument/2006/math'

def make_omml(omml_xml):
    """Создаёт OMML элемент из XML строки."""
    nsmap = {
        'm': MATH_NS,
        'w': 'http://schemas.openxmlformats.org/wordprocessingml/2006/main',
    }
    # Оборачиваем в корневой элемент с namespace
    full_xml = f'<m:oMathPara xmlns:m="{MATH_NS}" xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">{omml_xml}</m:oMathPara>'
    element = etree.fromstring(full_xml.encode('utf-8'))
    return element

def replace_paragraph_with_formula(doc, search_text, omml_xml):
    """Ищет абзац с текстом и заменяет его на формулу."""
    for i, para in enumerate(doc.paragraphs):
        if search_text in para.text:
            # Очищаем абзац
            for run in para.runs:
                run.text = ''

            # Создаём OMML-формулу
            formula_el = make_omml(omml_xml)

            # Вставляем в абзац
            para._element.append(formula_el)
            para.alignment = WD_ALIGN_PARAGRAPH.CENTER
            para.paragraph_format.first_line_indent = Cm(0)
            print(f'  Заменена формула: {search_text[:50]}...')
            return True
    return False

# ==================== ФОРМУЛЫ ====================

# 1. Евклидово расстояние (глава 3.2/3.4)
euclidean = '''<m:oMath>
  <m:r><m:t>d</m:t></m:r>
  <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr>
    <m:e><m:r><m:t>x</m:t></m:r><m:r><m:t>,</m:t></m:r><m:r><m:t>y</m:t></m:r></m:e>
  </m:d>
  <m:r><m:t>=</m:t></m:r>
  <m:r><m:t>‖</m:t></m:r>
  <m:r><m:t>x</m:t></m:r>
  <m:r><m:t>−</m:t></m:r>
  <m:r><m:t>y</m:t></m:r>
  <m:r><m:t>‖</m:t></m:r>
  <m:sSub><m:e><m:r><m:t/></m:r></m:e><m:sub><m:r><m:t>2</m:t></m:r></m:sub></m:sSub>
  <m:r><m:t>=</m:t></m:r>
  <m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/><m:e>
    <m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>
      <m:sub><m:r><m:t>i=1</m:t></m:r></m:sub>
      <m:sup><m:r><m:t>128</m:t></m:r></m:sup>
      <m:e>
        <m:sSup><m:e>
          <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr>
            <m:e>
              <m:sSub><m:e><m:r><m:t>x</m:t></m:r></m:e><m:sub><m:r><m:t>i</m:t></m:r></m:sub></m:sSub>
              <m:r><m:t>−</m:t></m:r>
              <m:sSub><m:e><m:r><m:t>y</m:t></m:r></m:e><m:sub><m:r><m:t>i</m:t></m:r></m:sub></m:sSub>
            </m:e>
          </m:d>
        </m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup>
      </m:e>
    </m:nary>
  </m:e></m:rad>
</m:oMath>'''

# 2. Triplet Loss
triplet = '''<m:oMath>
  <m:r><m:t>L</m:t></m:r>
  <m:r><m:t>=</m:t></m:r>
  <m:r><m:t>max</m:t></m:r>
  <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr>
    <m:e>
      <m:r><m:t>‖</m:t></m:r>
      <m:r><m:t>f</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e><m:r><m:t>a</m:t></m:r></m:e></m:d>
      <m:r><m:t>−</m:t></m:r>
      <m:r><m:t>f</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e><m:r><m:t>p</m:t></m:r></m:e></m:d>
      <m:sSup><m:e><m:r><m:t>‖</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup>
      <m:r><m:t>−</m:t></m:r>
      <m:r><m:t>‖</m:t></m:r>
      <m:r><m:t>f</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e><m:r><m:t>a</m:t></m:r></m:e></m:d>
      <m:r><m:t>−</m:t></m:r>
      <m:r><m:t>f</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e><m:r><m:t>n</m:t></m:r></m:e></m:d>
      <m:sSup><m:e><m:r><m:t>‖</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup>
      <m:r><m:t>+</m:t></m:r>
      <m:r><m:t>α</m:t></m:r>
      <m:r><m:t>,</m:t></m:r>
      <m:r><m:t> 0</m:t></m:r>
    </m:e>
  </m:d>
</m:oMath>'''

# 3. ArcFace Loss
arcface = '''<m:oMath>
  <m:r><m:t>L</m:t></m:r>
  <m:r><m:t>=</m:t></m:r>
  <m:r><m:t>−</m:t></m:r>
  <m:r><m:t>log</m:t></m:r>
  <m:f><m:fPr/><m:num>
    <m:r><m:t>exp</m:t></m:r>
    <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e>
      <m:r><m:t>s</m:t></m:r>
      <m:r><m:t>·</m:t></m:r>
      <m:r><m:t>cos</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e>
        <m:sSub><m:e><m:r><m:t>θ</m:t></m:r></m:e><m:sub><m:r><m:t>y</m:t></m:r></m:sub></m:sSub>
        <m:r><m:t>+</m:t></m:r>
        <m:r><m:t>m</m:t></m:r>
      </m:e></m:d>
    </m:e></m:d>
  </m:num><m:den>
    <m:r><m:t>exp</m:t></m:r>
    <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e>
      <m:r><m:t>s</m:t></m:r>
      <m:r><m:t>·</m:t></m:r>
      <m:r><m:t>cos</m:t></m:r>
      <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e>
        <m:sSub><m:e><m:r><m:t>θ</m:t></m:r></m:e><m:sub><m:r><m:t>y</m:t></m:r></m:sub></m:sSub>
        <m:r><m:t>+</m:t></m:r>
        <m:r><m:t>m</m:t></m:r>
      </m:e></m:d>
    </m:e></m:d>
    <m:r><m:t>+</m:t></m:r>
    <m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>
      <m:sub><m:r><m:t>j≠y</m:t></m:r></m:sub>
      <m:sup><m:r><m:t/></m:r></m:sup>
      <m:e>
        <m:r><m:t>exp</m:t></m:r>
        <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr><m:e>
          <m:r><m:t>s</m:t></m:r>
          <m:r><m:t>·</m:t></m:r>
          <m:r><m:t>cos</m:t></m:r>
          <m:sSub><m:e><m:r><m:t>θ</m:t></m:r></m:e><m:sub><m:r><m:t>j</m:t></m:r></m:sub></m:sSub>
        </m:e></m:d>
      </m:e>
    </m:nary>
  </m:den></m:f>
</m:oMath>'''

# 4. Пороговая функция верификации
threshold = '''<m:oMath>
  <m:r><m:t>V</m:t></m:r>
  <m:d><m:dPr><m:begChr m:val="("/><m:endChr m:val=")"/></m:dPr>
    <m:e><m:r><m:t>x</m:t></m:r><m:r><m:t>,</m:t></m:r><m:r><m:t>y</m:t></m:r><m:r><m:t>,</m:t></m:r><m:r><m:t>t</m:t></m:r></m:e>
  </m:d>
  <m:r><m:t>=</m:t></m:r>
  <m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr>
    <m:e>
      <m:eqArr>
        <m:e><m:r><m:t>1,  если  d(x, y) &lt; t</m:t></m:r></m:e>
        <m:e><m:r><m:t>0,  иначе</m:t></m:r></m:e>
      </m:eqArr>
    </m:e>
  </m:d>
</m:oMath>'''

print('Замена формул...')

# Ищем и заменяем текстовые формулы
replace_paragraph_with_formula(doc,
    'd(a, b) = sqrt(sum((a_i - b_i)^2)), i = 1..128',
    euclidean)

replace_paragraph_with_formula(doc,
    'L = max(||f(a) - f(p)||',
    triplet)

replace_paragraph_with_formula(doc,
    'L = -log(exp(s',
    arcface)

replace_paragraph_with_formula(doc,
    'd(x, y) = ||x - y||_2',
    euclidean)

replace_paragraph_with_formula(doc,
    'V(x, y, t) = 1, если',
    threshold)

doc.save(path)
print(f'Формулы обновлены: {path}')
