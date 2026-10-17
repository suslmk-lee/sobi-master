import React from 'react'
import {createRoot} from 'react-dom/client'
// 글꼴을 앱에 내장한다. PC 에 Pretendard 가 없으면 맑은 고딕으로 떨어져 제목이 가늘고
// 거칠게 보인다. 동적 서브셋이라 화면에 쓰인 글자 범위의 조각만 불러온다(인터넷 불필요).
import 'pretendard/dist/web/variable/pretendardvariable-dynamic-subset.css'
import './style.css'
import App from './App'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <App/>
    </React.StrictMode>
)
