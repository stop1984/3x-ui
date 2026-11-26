import { useCallback, useEffect, useRef, useState } from 'react';
import { Form } from 'antd';

import { FinalMaskForm } from '@/lib/xray/forms/transport';
import type { FinalMaskStreamSettings } from '@/schemas/protocols/stream/finalmask';

interface FinalMaskFieldProps {
  value?: FinalMaskStreamSettings;
  onChange?: (next: FinalMaskStreamSettings) => void;
  network: string;
  protocol: string;
  security?: string;
  showAll?: boolean;
  side?: 'server' | 'client';
}

const EMPTY: FinalMaskStreamSettings = { tcp: [], udp: [] };

export default function FinalMaskField({
  value,
  onChange,
  network,
  protocol,
  security,
  showAll,
  side,
}: FinalMaskFieldProps) {
  const [form] = Form.useForm();
  const [initial] = useState(() => value ?? EMPTY);
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  });
  const lastEmitted = useRef(JSON.stringify(initial));

  const emitFinalMask = useCallback(() => {
    const next = form.getFieldValue('finalmask') as FinalMaskStreamSettings | undefined;
    if (next === undefined) return;
    const serialized = JSON.stringify(next);
    if (serialized === lastEmitted.current) return;
    lastEmitted.current = serialized;
    onChangeRef.current?.(next);
  }, [form]);

  // AntD's useWatch is deferred. Emit user edits before the outer form can
  // submit, and keep watching for programmatic changes such as regeneration.
  const finalmask = Form.useWatch('finalmask', form);
  useEffect(() => {
    if (finalmask !== undefined) emitFinalMask();
  }, [finalmask, emitFinalMask]);

  return (
    <Form
      form={form}
      component={false}
      colon={false}
      labelCol={{ sm: { span: 8 } }}
      wrapperCol={{ sm: { span: 14 } }}
      labelWrap
      initialValues={{ finalmask: initial }}
      onValuesChange={emitFinalMask}
    >
      <FinalMaskForm
        name="finalmask"
        network={network}
        protocol={protocol}
        security={security}
        form={form}
        showAll={showAll}
        side={side}
      />
    </Form>
  );
}
